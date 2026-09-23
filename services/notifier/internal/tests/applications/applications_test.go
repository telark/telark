package applications

import (
	"context"
	"net/http"
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/subscribers/applications"
	"github.com/telark/notifier/internal/tests/testutil"
	"github.com/telark/rest/response"
	natscore "github.com/telark/x-ware/nats/core"
)

const (
	updateSubject = "telark.applications.update"
	deleteSubject = "telark.applications.delete"
)

type fakeAppClient struct {
	patchStatus, createStatus, deleteStatus int
	patched, created, deleted               bool
}

func (f *fakeAppClient) PatchApplicationByName(_ string, _ map[string]any) *response.GenericResponse {
	f.patched = true
	return &response.GenericResponse{Status: f.patchStatus}
}

func (f *fakeAppClient) CreateApplication(_ *appresource.Application) *response.GenericResponse {
	f.created = true
	return &response.GenericResponse{Status: f.createStatus}
}

func (f *fakeAppClient) DeleteApplicationByName(_ string) *response.GenericResponse {
	f.deleted = true
	return &response.GenericResponse{Status: f.deleteStatus}
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
// backend error is acked-and-logged. A delete removes it. All routed through the
// injected client so no exporter is needed.
func TestApplicationSubscriberRoutesEvents(t *testing.T) {
	cases := []struct {
		name                     string
		subject                  string
		payload                  []byte
		fake                     fakeAppClient
		wantPatched, wantCreated bool
		wantDeleted              bool
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
		{"delete removed", deleteSubject, deleteMsg(), fakeAppClient{deleteStatus: http.StatusOK}, false, false, true},
		{"delete backend error acked", deleteSubject, deleteMsg(), fakeAppClient{deleteStatus: http.StatusInternalServerError}, false, false, true},
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
			testutil.Equal(t, "deleted", fake.deleted, c.wantDeleted)
		})
	}
}
