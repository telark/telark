package passkey

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/constants"
	passkeyhandler "github.com/telark/auth/internal/handlers/passkey"
	"github.com/telark/auth/internal/tests/testutil"
	"github.com/telark/rest/base"
	autheps "github.com/telark/rest/endpoints/auth"
	"github.com/telark/rest/router"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	attackerUserID = "attacker-user"
	victimUserID   = "victim-user"
	credentialID   = "victim-credential"
	sessionToken   = "session-token"
	serviceToken   = "service-token"
	orphanBody     = `{"forceLastDelete":true,"cleanupOrphaned":true}`
	plainBody      = `{"forceLastDelete":true}`
)

func deleteRequest(body string, identity *xauthz.Identity, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodDelete, "/", strings.NewReader(body))
	r.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	if identity != nil {
		r = r.WithContext(xauthz.WithIdentity(r.Context(), *identity))
	}
	return r
}

// The resource backend is deliberately absent, so a request that reaches it
// answers 500: that is what separates "was allowed to act on this user" from
// "was refused before acting".
func TestDeletePasskeyOrphanCleanupBindsToIdentity(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		identity *xauthz.Identity
		headers  map[string]string
		want     int
	}{
		{
			name: "unauthenticated caller cannot name another user",
			body: orphanBody,
			headers: map[string]string{
				constants.HeaderUserID:       victimUserID,
				constants.HeaderCredentialID: credentialID,
			},
			want: http.StatusUnauthorized,
		},
		{
			name:     "session user cannot name another user",
			body:     orphanBody,
			identity: &xauthz.Identity{UserID: attackerUserID},
			headers: map[string]string{
				constants.HeaderUserID:       victimUserID,
				constants.HeaderCredentialID: credentialID,
				constants.HeaderSessionToken: sessionToken,
			},
			want: http.StatusUnauthorized,
		},
		{
			name:     "internal caller may clean up an orphan",
			body:     orphanBody,
			identity: &xauthz.Identity{Internal: true},
			headers: map[string]string{
				constants.HeaderUserID:       victimUserID,
				constants.HeaderCredentialID: credentialID,
			},
			want: http.StatusInternalServerError,
		},
		{
			name:     "internal caller without a user id",
			body:     orphanBody,
			identity: &xauthz.Identity{Internal: true},
			headers:  map[string]string{constants.HeaderCredentialID: credentialID},
			want:     http.StatusBadRequest,
		},
		{
			name:     "internal caller without a credential id",
			body:     orphanBody,
			identity: &xauthz.Identity{Internal: true},
			headers:  map[string]string{constants.HeaderUserID: victimUserID},
			want:     http.StatusBadRequest,
		},
		{
			name:     "ordinary delete still validates the session",
			body:     plainBody,
			identity: &xauthz.Identity{UserID: attackerUserID},
			headers:  map[string]string{constants.HeaderCredentialID: credentialID},
			want:     http.StatusUnauthorized,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			passkeyhandler.DeletePasskey(rec, deleteRequest(c.body, c.identity, c.headers))
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
	probe := func(_ http.ResponseWriter, r *http.Request) {
		seenUserID = r.Header.Get(constants.HeaderUserID)
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
		constants.HeaderCredentialID: credentialID,
		constants.HeaderSessionToken: sessionToken,
	})
	r.URL.Path = router.Pattern(endpoint)
	mux.ServeHTTP(httptest.NewRecorder(), r)

	testutil.Equal(t, "user id seen by handler", seenUserID, attackerUserID)
	testutil.Equal(t, "internal", seenInternal, false)
}
