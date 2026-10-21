package authz

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
)

type noSessions struct{}

func (noSessions) UserIDForToken(string) (string, error)       { return "", xauthz.ErrNotFound }
func (noSessions) GrantsForUser(string) (xauthz.Grants, error) { return xauthz.Grants{}, nil }

// Two session cookies count as no session: logout, a Public route, still runs and clears the host-only cookie, so it
// stays the way out, and a route that needs a session answers 401 with the code the dashboard explains.
func TestDuplicateSessionCookies(t *testing.T) {
	middleware, err := xauthz.New(xauthz.Config{Resolver: noSessions{}, Requirements: authz.Requirements(),
		RouteKey: router.KeyFromRequest, ServiceToken: "service-token"})
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	rt := router.NewRouter([]router.Route{
		router.CreateRoute(base.Post, autheps.Logout, authhandler.Logout),
		router.CreateRoute(base.Get, autheps.GetMyPermissions, func(http.ResponseWriter, *http.Request) { reached = true }),
	})
	rt.Use(middleware)
	send := func(method string, endpoint base.Endpoint) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, router.Pattern(endpoint), nil)
		r.Header.Set(dataconstants.HeaderRequestedWith, "XMLHttpRequest")
		for _, value := range []string{"real", "planted"} {
			r.AddCookie(&http.Cookie{Name: dataconstants.CookieSession, Value: value, Secure: true, HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
		}
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, r)
		return rec
	}
	logout := send(http.MethodPost, autheps.Logout)
	cleared := slices.ContainsFunc(logout.Result().Cookies(), func(c *http.Cookie) bool {
		return c.Name == dataconstants.CookieSession && c.MaxAge < 0
	})
	if logout.Code != http.StatusOK || !cleared {
		t.Fatalf("logout = %d, cookies %+v, want 200 clearing %s", logout.Code, logout.Result().Cookies(), dataconstants.CookieSession)
	}
	permissions := send(http.MethodGet, autheps.GetMyPermissions)
	if permissions.Code != http.StatusUnauthorized || reached || !strings.Contains(permissions.Body.String(), "session_cookie_duplicate") {
		t.Fatalf("permissions = %d %s, reached %v, want 401 session_cookie_duplicate", permissions.Code, permissions.Body.String(), reached)
	}
}
