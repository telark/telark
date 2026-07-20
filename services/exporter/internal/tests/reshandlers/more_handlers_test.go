package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	if body == "" {
		r = httptest.NewRequest(method, url, nil)
	} else {
		r = httptest.NewRequest(method, url, strings.NewReader(body))
	}
	r.Header.Set(constants.HeaderUserID, "u1")
	r.Header.Set(constants.HeaderCredentialID, "c1")
	return mux.SetURLVars(r, map[string]string{
		"id": "val", "name": "val", "userId": "u1", "groupId": "g1",
		"credentialId": "c1", "token": "tok", "scope": "roles",
	})
}

func TestApplicationHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, apphandler.GetApplicationResourceWithCacheInvalidation(o), varsReq(http.MethodGet, ""), "GetApplication")
	assertErrorResponse(t, apphandler.ListApplicationResourcesWithCacheInvalidation(o), varsReq(http.MethodGet, ""), "ListApplications")
	assertErrorResponse(t, apphandler.DeleteApplicationResourceWithCacheInvalidation(o), varsReq(http.MethodDelete, ""), "DeleteApplication")
	assertErrorResponse(t, apphandler.GetRollbacks(), varsReq(http.MethodGet, ""), "GetRollbacks")
	assertErrorResponse(t, apphandler.GetRollback(), varsReq(http.MethodGet, ""), "GetRollback")
}

func TestGlobalConfigHandlers(t *testing.T) {
	assertErrorResponse(t, globalconfighandler.GetGlobalConfig(), varsReq(http.MethodGet, ""), "GetGlobalConfig")
}

func TestSessionHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, sessionhandler.CreateSessionByUserWithCacheInvalidation(o), varsReq(http.MethodPost, "{}"), "CreateSession")
	assertErrorResponse(t, sessionhandler.ListSessionsByUserWithCacheInvalidation(), varsReq(http.MethodGet, ""), "ListSessions")
	assertErrorResponse(t, sessionhandler.GetSessionByToken(), varsReq(http.MethodGet, ""), "GetSessionByToken")
	assertErrorResponse(t, sessionhandler.PatchSessionByTokenWithCacheInvalidation(o), varsReq(http.MethodPatch, "{}"), "PatchSession")
	assertErrorResponse(t, sessionhandler.DeleteSessionByTokenWithCacheInvalidation(o), varsReq(http.MethodDelete, ""), "DeleteSession")
}

func TestPasskeyHandlers(t *testing.T) {
	o := newOptimizer(t)
	assertErrorResponse(t, passkeyhandler.CreatePasskeyByUserWithCacheInvalidation(o), varsReq(http.MethodPost, "{}"), "CreatePasskey")
	assertErrorResponse(t, passkeyhandler.ListPasskeysByUserWithCacheInvalidation(), varsReq(http.MethodGet, ""), "ListPasskeys")
	assertErrorResponse(t, passkeyhandler.GetPasskeyByUserAndCredentialIDWithCacheInvalidation(), varsReq(http.MethodGet, ""), "GetPasskey")
	assertErrorResponse(t, passkeyhandler.PatchPasskeyByUserAndCredentialIDWithCacheInvalidation(o), varsReq(http.MethodPatch, "{}"), "PatchPasskey")
	assertErrorResponse(t, passkeyhandler.DeletePasskeyByUserAndCredentialIDWithCacheInvalidation(o), varsReq(http.MethodDelete, ""), "DeletePasskey")
}
