package reshandlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/constants"
	passkeyhandler "github.com/telark/exporter/internal/handlers/auth/passkey"
	sessionhandler "github.com/telark/exporter/internal/handlers/auth/session"
	apphandler "github.com/telark/exporter/internal/handlers/resources/application"
	globalconfighandler "github.com/telark/exporter/internal/handlers/resources/globalconfig"
)

// varsReq carries every path var, header and query the auth/resource handlers
// might read, so each proceeds past request parsing to the (absent) backend.
func varsReq(method, body string) *http.Request {
	url := "/x/val?userId=u1&scope=roles&username=n&email=e@x.io"
	var r *http.Request
	if body == constants.EmptyString {
		r = httptest.NewRequest(method, url, nil)
	} else {
		r = httptest.NewRequest(method, url, strings.NewReader(body))
	}
	r.Header.Set(constants.HeaderUserID, testUserID)
	r.Header.Set(constants.HeaderCredentialID, testCategoryID)
	return mux.SetURLVars(r, map[string]string{
		constants.IDParam: "val", constants.NameParam: "val", constants.UserIDParam: testUserID, constants.GroupIDParam: testGroupID,
		"credentialId": testCategoryID, constants.TokenParam: "tok", constants.ScopeParam: constants.ResourceRole,
	})
}

func TestApplicationHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, apphandler.GetApplicationResourceWithCacheInvalidation(), varsReq(http.MethodGet, constants.EmptyString), "GetApplication")
	assertErrorResponse(t, apphandler.ListApplicationResourcesWithCacheInvalidation(),
		varsReq(http.MethodGet, constants.EmptyString), "ListApplications")
	assertErrorResponse(t, apphandler.DeleteApplicationResourceWithCacheInvalidation(o),
		varsReq(http.MethodDelete, constants.EmptyString), "DeleteApplication")
	assertErrorResponse(t, apphandler.GetRollbacks(), varsReq(http.MethodGet, constants.EmptyString), "GetRollbacks")
	assertErrorResponse(t, apphandler.GetRollback(), varsReq(http.MethodGet, constants.EmptyString), "GetRollback")
}

// Long enough for a fire-and-forget goroutine to reach the transport.
const outboundWindow = 300 * time.Millisecond

type recordingTransport chan string

func (r recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r <- req.URL.String()
	return nil, errors.New("outbound call")
}

// The exporter owns the CR and never calls another service: a DELETE that called
// back into discovery's reset looped, since that reset is what calls this DELETE.
func TestDeleteApplicationMakesNoOutboundCall(t *testing.T) {
	calls := make(recordingTransport, constants.DefaultChannelBufferSize)
	original := http.DefaultTransport
	http.DefaultTransport = calls
	t.Cleanup(func() { http.DefaultTransport = original })

	assertErrorResponse(t, apphandler.DeleteApplicationResourceWithCacheInvalidation(newOptimizer(t)),
		varsReq(http.MethodDelete, constants.EmptyString), "DeleteApplication")

	select {
	case url := <-calls:
		t.Fatalf("DELETE called %s", url)
	case <-time.After(outboundWindow):
	}
}

func TestGlobalConfigHandlers(t *testing.T) {
	assertErrorResponse(t, globalconfighandler.GetGlobalConfig(), varsReq(http.MethodGet, constants.EmptyString), "GetGlobalConfig")
}

func TestSessionHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, sessionhandler.CreateSessionByUserWithCacheInvalidation(o), varsReq(http.MethodPost, emptyJSONBody), "CreateSession")
	assertErrorResponse(t, sessionhandler.ListSessionsByUserWithCacheInvalidation(), varsReq(http.MethodGet, constants.EmptyString), "ListSessions")
	assertErrorResponse(t, sessionhandler.GetSessionByToken(), varsReq(http.MethodGet, constants.EmptyString), "GetSessionByToken")
	assertErrorResponse(t, sessionhandler.PatchSessionByTokenWithCacheInvalidation(o), varsReq(http.MethodPatch, emptyJSONBody), "PatchSession")
	assertErrorResponse(t, sessionhandler.DeleteSessionByTokenWithCacheInvalidation(o),
		varsReq(http.MethodDelete, constants.EmptyString), "DeleteSession")
}

func TestPasskeyHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, passkeyhandler.CreatePasskeyByUserWithCacheInvalidation(o), varsReq(http.MethodPost, emptyJSONBody), "CreatePasskey")
	assertErrorResponse(t, passkeyhandler.ListPasskeysByUserWithCacheInvalidation(), varsReq(http.MethodGet, constants.EmptyString), "ListPasskeys")
	assertErrorResponse(t, passkeyhandler.GetPasskeyByUserAndCredentialIDWithCacheInvalidation(),
		varsReq(http.MethodGet, constants.EmptyString), "GetPasskey")
	assertErrorResponse(t, passkeyhandler.PatchPasskeyByUserAndCredentialIDWithCacheInvalidation(o),
		varsReq(http.MethodPatch, emptyJSONBody), "PatchPasskey")
	assertErrorResponse(t, passkeyhandler.DeletePasskeyByUserAndCredentialIDWithCacheInvalidation(o),
		varsReq(http.MethodDelete, constants.EmptyString), "DeletePasskey")
}
