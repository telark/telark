package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/auth/internal/constants"
	cleanuphandler "github.com/telark/auth/internal/handlers/cleanup"
	"github.com/telark/rest/base"
	restconstants "github.com/telark/rest/constants"
	autheps "github.com/telark/rest/endpoints/auth"
	"github.com/telark/rest/router"
)

// Routed through the real mux, so the id is read under the name the route
// template registers it with. With it present the handler moves past parameter
// validation to the (absent) backend instead of answering 400.
func TestDeleteUserCleanupReadsIDFromRoute(t *testing.T) {
	endpoint := autheps.DeleteUserCleanup
	mux := router.NewRouter([]router.Route{
		router.CreateRoute(base.Delete, endpoint, cleanuphandler.DeleteUser),
	})

	path := strings.Replace(router.Pattern(endpoint), restconstants.IDParam, "u-1", constants.DefaultIncrementValue)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("id path parameter not read: %d %s", rec.Code, rec.Body.String())
	}
}
