package applications

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/internal/data/resources/shared"
	restconstants "github.com/telark/telark/internal/rest/constants"
	"github.com/telark/telark/internal/rest/response"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/services/notifier/internal/subscribers/applications"
	"github.com/telark/telark/services/notifier/internal/tests/testutil"
)

const (
	updateSubject = "telark.applications.update"
	deleteSubject = "telark.applications.delete"
	ackPayload    = "+ACK"
	nakPayload    = "-NAK"
	replyWait     = 2 * time.Second
)

type fakeAppClient struct {
	patchStatus, createStatus int
	resetResp                 *response.GenericResponse
	resetErr                  error
	patched, created, reset   bool
}

func (f *fakeAppClient) PatchApplicationByName(_ string, _ map[string]any) *response.GenericResponse {
	f.patched = true
	return &response.GenericResponse{Status: f.patchStatus}
}

func (f *fakeAppClient) CreateApplication(_ *appresource.Application) *response.GenericResponse {
	f.created = true
	return &response.GenericResponse{Status: f.createStatus}
}

func (f *fakeAppClient) ResetApplicationByName(_ string) (*response.GenericResponse, error) {
	f.reset = true
	return f.resetResp, f.resetErr
}

func statusErr(status int) error {
	return fmt.Errorf(string(restconstants.ErrUnexpectedStatus), status, "boom")
}

func updateMsg() []byte {
	return testutil.Data(&natscore.Message{
		ResourceName: "app1", ResourceType: shared.Application,
		Scope: string(shared.ApplicationSpecScope), Data: map[string]any{"replicas": 1},
	})
}

func deleteMsg() []byte {
	return testutil.Data(&natscore.Message{ResourceName: "app1", ResourceType: shared.Application})
}

func updateNoScopeMsg() []byte {
	return testutil.Data(&natscore.Message{
		ResourceName: "app1", ResourceType: shared.Application, Data: map[string]any{"replicas": 1},
	})
}

// An update patches the application CR; a 404 upgrades to a create (upsert); a
// backend error is acked-and-logged. A delete resets it through discovery. All
// routed through the injected client so no exporter or discovery is needed.
func TestApplicationSubscriberRoutesEvents(t *testing.T) {
	cases := []struct {
		name                     string
		subject                  string
		payload                  []byte
		fake                     fakeAppClient
		wantPatched, wantCreated bool
		wantReset                bool
	}{
		{"update patched", updateSubject, updateMsg(), fakeAppClient{patchStatus: http.StatusOK}, true, false, false},
		{
			"update upserts on 404", updateSubject, updateMsg(),
			fakeAppClient{patchStatus: http.StatusNotFound, createStatus: http.StatusOK}, true, true, false,
		},
		{"update backend error acked", updateSubject, updateMsg(), fakeAppClient{patchStatus: http.StatusInternalServerError}, true, false, false},
		{
			"update create fails on 404", updateSubject, updateMsg(),
			fakeAppClient{patchStatus: http.StatusNotFound, createStatus: http.StatusInternalServerError}, true, true, false,
		},
		{"update missing scope acked", updateSubject, updateNoScopeMsg(), fakeAppClient{}, false, false, false},
		{
			"delete resets", deleteSubject, deleteMsg(),
			fakeAppClient{resetResp: &response.GenericResponse{Status: http.StatusOK}}, false, false, true,
		},
		{"delete reset error", deleteSubject, deleteMsg(), fakeAppClient{resetErr: statusErr(http.StatusBadGateway)}, false, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := c.fake
			sub := applications.NewApplicationSubscriberWithClient(&fake)
			sub.RetryDelay = 0 // synthetic messages can't be acked; skip the retry backoff
			sub.MaxRetries = 1
			testutil.Equal(t, "resource type", sub.GetResourceType(), shared.Application)

			_ = sub.ProcessMessage(context.Background(), testutil.RawMsg(c.subject, c.payload))

			testutil.Equal(t, "patched", fake.patched, c.wantPatched)
			testutil.Equal(t, "created", fake.created, c.wantCreated)
			testutil.Equal(t, "reset", fake.reset, c.wantReset)
		})
	}
}

// boundDeleteMsg is a delete whose ack or nak lands on a subject the test reads.
func boundDeleteMsg(t *testing.T) (*nats.Msg, *nats.Subscription) {
	t.Helper()
	c := testutil.NatsServer(t)
	inbox := nats.NewInbox()
	replies, err := c.Conn.SubscribeSync(inbox)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := c.Conn.SubscribeSync(nats.NewInbox())
	if err != nil {
		t.Fatal(err)
	}
	m := testutil.RawMsg(deleteSubject, deleteMsg())
	m.Reply, m.Sub = inbox, bound
	return m, replies
}

// A gone application is success, a reset that may succeed later (discovery or the
// exporter down, a timeout) is redelivered, and a refusal is acked and logged.
func TestDeleteAckNakMapping(t *testing.T) {
	cases := []struct {
		name  string
		fake  fakeAppClient
		reply string
	}{
		{"reset ok", fakeAppClient{resetResp: &response.GenericResponse{Status: http.StatusOK}}, ackPayload},
		{"already gone", fakeAppClient{resetErr: statusErr(http.StatusNotFound)}, ackPayload},
		{"exporter delete failed", fakeAppClient{resetErr: statusErr(http.StatusBadGateway)}, nakPayload},
		{"discovery unavailable", fakeAppClient{resetErr: statusErr(http.StatusServiceUnavailable)}, nakPayload},
		{"no http answer", fakeAppClient{resetErr: errors.New("context deadline exceeded")}, nakPayload},
		{"forbidden", fakeAppClient{resetErr: statusErr(http.StatusForbidden)}, ackPayload},
		{"unauthorized", fakeAppClient{resetErr: statusErr(http.StatusUnauthorized)}, ackPayload},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := c.fake
			sub := applications.NewApplicationSubscriberWithClient(&fake)
			m, replies := boundDeleteMsg(t)

			_ = sub.ProcessMessage(context.Background(), m)

			reply, err := replies.NextMsg(replyWait)
			if err != nil {
				t.Fatalf("no ack or nak: %v", err)
			}
			testutil.Equal(t, "reset", fake.reset, true)
			testutil.Equal(t, "reply", strings.HasPrefix(string(reply.Data), c.reply), true)
		})
	}
}
