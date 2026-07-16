package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/rest/base"
	"github.com/telark/rest/router"
)

const testEndpoint base.Endpoint = "resources/users/{id}/patch"

func noopHandler(_ http.ResponseWriter, _ *http.Request) {}

func TestKeyMatchesRegisteredPattern(t *testing.T) {
	route := router.CreateRoute(base.Patch, testEndpoint, noopHandler)

	want := route.Method + " " + route.Pattern
	if got := router.Key(base.Patch, testEndpoint); got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}

// The two must agree exactly: a lookup table built with Key is read back with
// KeyFromRequest, and any drift silently denies every request.
func TestKeyFromRequestMatchesKey(t *testing.T) {
	routes := []router.Route{router.CreateRoute(base.Patch, testEndpoint, noopHandler)}

	var seen string
	mux := router.NewRouter(routes)
	mux.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = router.KeyFromRequest(r)
			next.ServeHTTP(w, r)
		})
	})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/resources/users/u-1/patch", nil)
	mux.ServeHTTP(httptest.NewRecorder(), req)

	want := router.Key(base.Patch, testEndpoint)
	if seen != want {
		t.Errorf("KeyFromRequest() = %q, want %q", seen, want)
	}
}

func TestKeyFromRequestWithoutMatchIsEmpty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nothing", nil)

	if got := router.KeyFromRequest(req); got != "" {
		t.Errorf("KeyFromRequest() = %q, want empty for an unmatched request", got)
	}
}

func TestPatternIsRooted(t *testing.T) {
	pattern := router.Pattern(testEndpoint)

	if want := "/api/v1/resources/users/{id}/patch"; pattern != want {
		t.Errorf("Pattern() = %q, want %q", pattern, want)
	}
}
