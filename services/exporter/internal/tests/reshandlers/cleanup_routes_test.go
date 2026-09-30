package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/rest/base"
	restconstants "github.com/telark/telark/internal/rest/constants"
	cleanupendpoints "github.com/telark/telark/internal/rest/endpoints/cleanup"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/exporter/internal/constants"
	cleanuphandler "github.com/telark/telark/services/exporter/internal/handlers/resources/cleanup"
)

// Routed through the real mux, so the handler reads the path variable by the name
// the route template registers it under. An unknown type must reach the type
// lookup rather than fail as a missing parameter.
func TestListCleanupViewsReadsTypeFromRoute(t *testing.T) {
	endpoint := cleanupendpoints.ListCleanupViews
	mux := router.NewRouter([]router.Route{
		router.CreateRoute(base.Get, endpoint, cleanuphandler.ListCleanupViews),
	})

	path := strings.Replace(router.Pattern(endpoint), restconstants.TypeParam, "unknown-type", constants.DefaultIncrementValue)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "unknown resource type") {
		t.Fatalf("want 400 unknown resource type, got %d %s", rec.Code, rec.Body.String())
	}
}
