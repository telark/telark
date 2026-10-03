package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/resources/finalizers"
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

// The body carries only the finalizer name: an extra key was accepted silently, an oversized body answered 422.
func TestFinalizerBodyIsStrictAndBounded(t *testing.T) {
	endpoint := cleanupendpoints.AddFinalizer
	mux := router.NewRouter([]router.Route{router.CreateRoute(base.Update, endpoint, cleanuphandler.AddFinalizer)})
	path := strings.NewReplacer(restconstants.TypeParam, finalizers.ResourceTypeUsers, restconstants.IDParam, testUserID).
		Replace(router.Pattern(endpoint))
	tests := []struct {
		name string
		body string
		want int
	}{
		{"unknown key", `{"name":"` + finalizers.UserCleanup + `","force":true}`, http.StatusBadRequest},
		{"oversized", `{"name":"` + strings.Repeat("x", int(base.MaxRequestBodySize)) + `"}`, http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader(tt.body)))
			if rec.Code != tt.want {
				t.Fatalf("code = %d %s, want %d", rec.Code, rec.Body.String(), tt.want)
			}
		})
	}
}
