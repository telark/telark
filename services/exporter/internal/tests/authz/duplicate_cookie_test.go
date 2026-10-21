package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/base"
	authendpoints "github.com/telark/telark/internal/rest/endpoints/auth"
	statusendpoints "github.com/telark/telark/internal/rest/endpoints/status"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
)

// Two session cookies count as no session: a Public route still runs, and one that needs a session, such as the
// self session the dashboard's session check reads, answers 401 with the code the dashboard explains.
func TestDuplicateSessionCookies(t *testing.T) {
	middleware, err := xauthz.New(xauthz.Config{Resolver: stubResolver{}, Requirements: authz.Requirements(),
		RouteKey: router.KeyFromRequest, ServiceToken: testServiceToken})
	if err != nil {
		t.Fatal(err)
	}
	rt := router.NewRouter([]router.Route{
		router.CreateRoute(base.Get, statusendpoints.LivenessCheck, sentinel),
		router.CreateRoute(base.Get, authendpoints.GetSelfSession, sentinel),
	})
	rt.Use(middleware)
	for endpoint, want := range map[base.Endpoint]int{
		statusendpoints.LivenessCheck: http.StatusOK, authendpoints.GetSelfSession: http.StatusUnauthorized,
	} {
		r := httptest.NewRequest(http.MethodGet, router.Pattern(endpoint), nil)
		for _, value := range []string{"real", "planted"} {
			r.AddCookie(&http.Cookie{Name: dataconstants.CookieSession, Value: value, Secure: true, HttpOnly: true,
				SameSite: http.SameSiteStrictMode})
		}
		rec := httptest.NewRecorder()
		rt.ServeHTTP(rec, r)
		if rec.Code != want || (want == http.StatusUnauthorized) != strings.Contains(rec.Body.String(), "session_cookie_duplicate") {
			t.Fatalf("%s = %d %s, want %d", endpoint, rec.Code, rec.Body.String(), want)
		}
	}
}
