package passkey

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/rest/base"
	restconstants "github.com/telark/telark/internal/rest/constants"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	attackerUserID = "attacker-user"
	victimUserID   = "victim-user"
	credentialID   = "victim-credential"
	sessionToken   = "session-token"
	serviceToken   = "service-token"
	orphanBody     = `{"forceLastDelete":true,"cleanupOrphaned":true}`
	plainBody      = `{"forceLastDelete":true}`
	oneCall        = 1
)

func deleteRequest(body string, identity *xauthz.Identity, headers map[string]string, credential string) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(body))
	if credential != "" {
		r = testutil.WithCredentialID(r, credential)
	}
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	if identity != nil {
		r = r.WithContext(xauthz.WithIdentity(r.Context(), *identity))
	}
	return r
}

type deleteCase struct {
	name       string
	body       string
	identity   *xauthz.Identity
	headers    map[string]string
	credential string
	want       int
}

var orphanCleanupCases = []deleteCase{
	{
		name: "unauthenticated caller cannot name another user",
		body: orphanBody,
		headers: map[string]string{
			constants.HeaderUserID: victimUserID,
		},
		credential: credentialID,
		want:       http.StatusUnauthorized,
	},
	{
		name:     "session user cannot name another user",
		body:     orphanBody,
		identity: &xauthz.Identity{UserID: attackerUserID},
		headers: map[string]string{
			constants.HeaderUserID:       victimUserID,
			constants.HeaderSessionToken: sessionToken,
		},
		credential: credentialID,
		want:       http.StatusServiceUnavailable,
	},
	{
		name:     "internal caller may clean up an orphan",
		body:     orphanBody,
		identity: &xauthz.Identity{Internal: true},
		headers: map[string]string{
			constants.HeaderUserID: victimUserID,
		},
		credential: credentialID,
		want:       http.StatusInternalServerError,
	},
	{
		name:       "internal caller without a user id",
		body:       orphanBody,
		identity:   &xauthz.Identity{Internal: true},
		credential: credentialID,
		want:       http.StatusBadRequest,
	},
	{
		name:     "internal caller without a credential id",
		body:     orphanBody,
		identity: &xauthz.Identity{Internal: true},
		headers:  map[string]string{constants.HeaderUserID: victimUserID},
		want:     http.StatusBadRequest,
	},
	{
		name:       "ordinary delete still validates the session",
		body:       plainBody,
		identity:   &xauthz.Identity{UserID: attackerUserID},
		credential: credentialID,
		want:       http.StatusUnauthorized,
	},
}

// The resource backend is deliberately absent, so a request that reaches it
// answers 500: that is what separates "was allowed to act on this user" from
// "was refused before acting".
func TestDeletePasskeyOrphanCleanupBindsToIdentity(t *testing.T) {
	for _, c := range orphanCleanupCases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			passkeyhandler.DeletePasskey(rec, deleteRequest(c.body, c.identity, c.headers, c.credential))
			testutil.Equal(t, "status", rec.Code, c.want)
		})
	}
}

type stubResolver struct{ userID string }

func (s stubResolver) UserIDForToken(string) (string, error) { return s.userID, nil }

func (stubResolver) GrantsForUser(string) (xauthz.Grants, error) { return xauthz.Grants{}, nil }

// The route's own middleware is the outer guard: it replaces a caller-supplied
// X-User-ID with the session's, so the handler never sees the spoofed value.
func TestDeletePasskeyRouteReplacesSpoofedUserID(t *testing.T) {
	endpoint := autheps.DeletePasskeyByUserAndCredentialIDViaProxy
	requirement := authz.Requirements()[router.Key(base.Delete, endpoint)]
	testutil.Equal(t, "access", requirement.Access, xauthz.AccessAuthenticated)

	var seenUserID string
	var seenInternal bool
	var seenCredential string
	probe := func(_ http.ResponseWriter, r *http.Request) {
		seenUserID = r.Header.Get(constants.HeaderUserID)
		seenCredential, _ = authhelper.ExtractCredentialID(r)
		identity, _ := xauthz.FromContext(r.Context())
		seenInternal = identity.Internal
	}

	middleware, err := xauthz.New(xauthz.Config{
		Resolver:     stubResolver{userID: attackerUserID},
		Requirements: authz.Requirements(),
		RouteKey:     router.KeyFromRequest,
		ServiceToken: serviceToken,
	})
	if err != nil {
		t.Fatalf("middleware: %v", err)
	}

	mux := router.NewRouter([]router.Route{router.CreateRoute(base.Delete, endpoint, probe)})
	mux.Use(middleware)

	r := deleteRequest(orphanBody, nil, map[string]string{
		constants.HeaderUserID:       victimUserID,
		constants.HeaderSessionToken: sessionToken,
	}, "")
	r.URL.Path = strings.ReplaceAll(router.Pattern(endpoint), restconstants.CredentialIDParam, credentialID)
	mux.ServeHTTP(httptest.NewRecorder(), r)

	testutil.Equal(t, "user id seen by handler", seenUserID, attackerUserID)
	testutil.Equal(t, "internal", seenInternal, false)
	testutil.Equal(t, "credential id from path", seenCredential, credentialID)
}

const (
	ownerUserID       = "owner-user"
	otherCredentialID = "other-credential"
	lastPasskeyMsg    = "cannot delete last passkey"
	notFoundMsg       = "passkey not found"
	sessionsPath      = "/auth/sessions/self"
	passkeysPath      = "/internal/auth/passkeys/"
	usersPath         = "/users/"
)

// passkeyExporter answers the session, passkey and user calls a passkey
// handler makes, and records the identities the delete path patches back.
type passkeyExporter struct {
	status  int
	message string
	user    userresource.User
	mu      sync.Mutex
	patched []any
}

func writeEnvelope(w http.ResponseWriter, status int, message string, data any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "message": message, "data": data})
}

func (e *passkeyExporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, sessionsPath):
		writeEnvelope(w, http.StatusOK, constants.EmptyString, authdata.Session{
			UserID:           ownerUserID,
			ExpiresTimestamp: time.Now().UTC().Add(time.Hour).Format(constants.TimeFormatRFC3339),
		})
	case strings.Contains(r.URL.Path, passkeysPath):
		writeEnvelope(w, e.status, e.message, nil)
	case strings.Contains(r.URL.Path, usersPath) && r.Method == http.MethodPatch:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.mu.Lock()
		e.patched = append(e.patched, body[constants.UserFieldIdentities])
		e.mu.Unlock()
		writeEnvelope(w, http.StatusOK, constants.EmptyString, nil)
	default:
		writeEnvelope(w, http.StatusOK, constants.EmptyString, e.user)
	}
}

func sessionRequest(method, body string) *http.Request {
	r := httptest.NewRequest(method, "/", strings.NewReader(body))
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	r.Header.Set(constants.HeaderSessionToken, sessionToken)
	return testutil.WithCredentialID(r, credentialID)
}

func responseMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	return body.Message
}

// The exporter's refusal reaches the caller with its own status and reason:
// an unknown credential is 404 on PATCH and DELETE, the last passkey is 400.
func TestPasskeyProxyRelaysExporterRefusals(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		status  int
		message string
	}{
		{"patch unknown credential", http.MethodPatch, http.StatusNotFound, notFoundMsg},
		{"delete unknown credential", http.MethodDelete, http.StatusNotFound, notFoundMsg},
		{"delete last passkey", http.MethodDelete, http.StatusBadRequest, lastPasskeyMsg},
		{"exporter failure stays opaque", http.MethodDelete, http.StatusInternalServerError,
			string(constants.ErrInternalServerError)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.StubBackend(t, &passkeyExporter{status: c.status, message: c.message})
			rec := httptest.NewRecorder()
			if c.method == http.MethodPatch {
				passkeyhandler.UpdatePasskey(rec, sessionRequest(c.method, `{"deviceName":"laptop"}`))
			} else {
				passkeyhandler.DeletePasskey(rec, sessionRequest(c.method, constants.EmptyString))
			}
			testutil.Equal(t, "status", rec.Code, c.status)
			testutil.Equal(t, "message", responseMessage(t, rec), c.message)
		})
	}
}

// Deleting a passkey drops the identity it was registered with and nothing else.
func TestDeletePasskeyDetachesItsIdentity(t *testing.T) {
	backend := &passkeyExporter{status: http.StatusOK, user: userresource.User{ID: ownerUserID, Identities: []*userresource.UserIdentity{
		{Provider: constants.IdentityProviderPasskey, Subject: credentialID},
		{Provider: constants.IdentityProviderPasskey, Subject: otherCredentialID},
		{Provider: constants.IdentityProviderGoogle, Subject: credentialID},
	}}}
	testutil.StubBackend(t, backend)

	rec := httptest.NewRecorder()
	passkeyhandler.DeletePasskey(rec, sessionRequest(http.MethodDelete, constants.EmptyString))
	testutil.Equal(t, "status", rec.Code, http.StatusOK)

	backend.mu.Lock()
	defer backend.mu.Unlock()
	testutil.Equal(t, "identity patches", len(backend.patched), oneCall)
	kept, _ := json.Marshal(backend.patched[0])
	want, _ := json.Marshal([]*userresource.UserIdentity{
		{Provider: constants.IdentityProviderPasskey, Subject: otherCredentialID},
		{Provider: constants.IdentityProviderGoogle, Subject: credentialID},
	})
	testutil.Equal(t, "kept identities", string(kept), string(want))
}
