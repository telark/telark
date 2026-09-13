package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cleanuphandler "github.com/telark/exporter/internal/handlers/resources/cleanup"
	"github.com/telark/rest/base"
	restconstants "github.com/telark/rest/constants"
	cleanupendpoints "github.com/telark/rest/endpoints/resources/cleanup"
	"github.com/telark/rest/router"
)

// Routed through the real mux, so the handler reads the path variable by the name
// the route template registers it under. An unknown type must reach the type
// lookup rather than fail as a missing parameter.
func TestListCleanupViewsReadsTypeFromRoute(t *testing.T) {
	endpoint := cleanupendpoints.ListCleanupViews
	mux := router.NewRouter([]router.Route{
		router.CreateRoute(base.Get, endpoint, cleanuphandler.ListCleanupViews),
	})

	path := strings.Replace(router.Pattern(endpoint), restconstants.TypeParam, "unknown-type", 1)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown resource type") {
		t.Fatalf("want 400 unknown resource type, got %d %s", rec.Code, rec.Body.String())
	}
}
