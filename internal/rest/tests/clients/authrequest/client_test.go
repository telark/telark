package authrequest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/authrequest"
	requestclient "github.com/telark/telark/internal/rest/clients/auth/request"
	"github.com/telark/telark/internal/rest/constants"
)

const (
	testServiceToken = "test-service-token"
	requestName      = "req-1"
	olderName        = "req-0"
	email            = "test@example.com"
	requestedBy      = "system:serviceaccount:telark:ops-admin"
	recipientKey     = "age1"
	reason           = "the bootstrap account denied the request"
	createdAt        = "2026-11-02T10:00:00Z"
	scheme           = "http"
	wantPending      = 2
	pendingPath      = "/api/v1/internal/auth/requests/pending"
	statusPath       = "/api/v1/internal/auth/requests/req-1/status"
	okBody           = `{"status":200,"operation":"Success"}`
	statusBody       = `{"phase":"Denied","reason":"the bootstrap account denied the request"}`
	pendingBody      = `{"status":200,"data":{"items":[` +
		`{"id":"req-0","email":"test@example.com","requestedBy":"system:serviceaccount:telark:ops-admin","recipientKey":"age1",` +
		`"phase":"Recovering","createdAt":"2026-11-02T10:00:00Z"},` +
		`{"id":"req-1","email":"test@example.com","requestedBy":"system:serviceaccount:telark:ops-admin","recipientKey":"age1"}]}}`
	refusedBody = `{"status":500,"operation":"Error","message":"listing the auth requests failed"}`
)

// The token is read once per process, so it must be set before any test in
// this binary sends its first request.
func init() {
	if err := os.Setenv(dataconstants.EnvServiceToken, testServiceToken); err != nil {
		panic(err)
	}
}

func serve(t *testing.T, status int, reply string) (*requestclient.Client, *captured) {
	t.Helper()
	got := &captured{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*got = captured{
			method: r.Method, path: r.URL.Path, token: r.Header.Get(dataconstants.HeaderServiceToken),
			cacheControl: r.Header.Get(constants.HeaderCacheControl), body: string(body),
		}
		w.WriteHeader(status)
		if _, err := w.Write([]byte(reply)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := requestclient.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = scheme
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
	return client, got
}

// A cached list would hand a finished request to the next run, so the read asks for a fresh one.
func TestPendingReadsTheListUncached(t *testing.T) {
	client, got := serve(t, http.StatusOK, pendingBody)

	pending, err := client.Pending()
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	want := captured{method: http.MethodGet, path: pendingPath, token: testServiceToken, cacheControl: constants.CacheControlNoCache}
	if *got != want {
		t.Fatalf("request = %+v, want %+v", *got, want)
	}
	if len(pending) != wantPending {
		t.Fatalf("pending = %+v, want two requests", pending)
	}
	older := authrequest.AuthRequest{
		ID:        olderName,
		Spec:      authrequest.Spec{Email: email, RequestedBy: requestedBy, RecipientKey: recipientKey},
		Status:    authrequest.Status{Phase: authrequest.PhaseRecovering},
		CreatedAt: createdAt,
	}
	if pending[dataconstants.DefaultInitValue] != older || !pending[dataconstants.DefaultInitValue].InFlight() {
		t.Errorf("first = %+v, want %+v", pending[dataconstants.DefaultInitValue], older)
	}
	if last := pending[dataconstants.SingleItem]; last.ID != requestName || last.Phase != dataconstants.EmptyString || last.InFlight() {
		t.Errorf("last = %+v, want %s with no phase", last, requestName)
	}
}

func TestPendingFailsOnARefusal(t *testing.T) {
	client, _ := serve(t, http.StatusInternalServerError, refusedBody)

	if pending, err := client.Pending(); err == nil || pending != nil {
		t.Fatalf("pending = %+v, %v, want an error and no list", pending, err)
	}
}

func TestSetStatusPatchesTheFieldsItSets(t *testing.T) {
	client, got := serve(t, http.StatusOK, okBody)

	resp := client.SetStatus(requestName, authrequest.Status{Phase: authrequest.PhaseDenied, Reason: reason})
	if resp == nil || resp.Status != http.StatusOK {
		t.Fatalf("expected status %d, got %+v", http.StatusOK, resp)
	}
	want := captured{method: http.MethodPatch, path: statusPath, token: testServiceToken, body: statusBody}
	if *got != want {
		t.Fatalf("request = %+v, want %+v", *got, want)
	}
}
