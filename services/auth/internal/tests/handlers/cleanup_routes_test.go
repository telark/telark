package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	restconstants "github.com/telark/telark/internal/rest/constants"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanuphandler "github.com/telark/telark/services/auth/internal/handlers/cleanup"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
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

func deleteRoleCleanup(t *testing.T, exporterStatus int, exporterBody string) (int, string) {
	t.Helper()
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(exporterStatus)
		_, _ = w.Write([]byte(exporterBody))
	}))
	endpoint := autheps.DeleteAccessRoleCleanup
	mux := router.NewRouter([]router.Route{
		router.CreateRoute(base.Delete, endpoint, cleanuphandler.DeleteAccessRole),
	})
	path := strings.Replace(router.Pattern(endpoint), restconstants.IDParam, "r-1", constants.DefaultIncrementValue)
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	mux.ServeHTTP(rec, r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{Internal: true})))

	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, rec.Body.String())
	}
	return rec.Code, body.Message
}

// A refused delete reaches the caller with the exporter's own message, not the
// rest client's "HTTP 403: {...}" wrapper around the raw body.
func TestDeleteCleanupForwardsExporterMessage(t *testing.T) {
	const protected = "this role is protected and cannot be deleted"
	code, msg := deleteRoleCleanup(t, http.StatusForbidden,
		`{"status":403,"operation":"Forbidden","message":"`+protected+`"}`)
	testutil.Equal(t, "status", code, http.StatusForbidden)
	testutil.Equal(t, "message", msg, protected)
}

// Nothing is left to clean for an object the exporter no longer has, so the
// caller learns that instead of a "deletion scheduled" for a job bound to fail.
func TestDeleteCleanupAnswersNotFoundWhenGone(t *testing.T) {
	const gone = "role not found"
	code, msg := deleteRoleCleanup(t, http.StatusNotFound, `{"status":404,"message":"`+gone+`"}`)
	testutil.Equal(t, "status", code, http.StatusNotFound)
	testutil.Equal(t, "message", msg, gone)
}

// Each auth cleanup path deletes through the exporter's plain resource path.
func TestCleanupRoutesDeleteThroughExporterPaths(t *testing.T) {
	cases := []struct {
		endpoint base.Endpoint
		handler  http.HandlerFunc
		path     string
		exporter string
	}{
		{autheps.DeleteUserCleanup, cleanuphandler.DeleteUser, "/api/v1/auth/users/x-1", "/api/v1/users/x-1"},
		{autheps.DeleteGroupCleanup, cleanuphandler.DeleteGroup, "/api/v1/auth/groups/x-1", "/api/v1/groups/x-1"},
		{autheps.DeleteAccessRoleCleanup, cleanuphandler.DeleteAccessRole, "/api/v1/auth/accessroles/x-1", "/api/v1/accessroles/x-1"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			var seen []string
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = append(seen, r.Method+" "+r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"status":404,"message":"gone"}`))
			}))
			mux := router.NewRouter([]router.Route{router.CreateRoute(base.Delete, c.endpoint, c.handler)})
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, c.path, nil)
			mux.ServeHTTP(rec, r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{Internal: true})))
			if !slices.Contains(seen, http.MethodDelete+" "+c.exporter) {
				t.Fatalf("exporter calls = %v, want DELETE %s", seen, c.exporter)
			}
		})
	}
}

// The exporter stamps a soft delete's lastUpdatedBy from the forwarded caller,
// so the delete must not arrive as an anonymous service call.
func TestCleanupDeletesForwardTheCaller(t *testing.T) {
	const deleter = "u-deleter"
	for _, c := range []struct {
		endpoint base.Endpoint
		handler  http.HandlerFunc
		path     string
	}{
		{autheps.DeleteUserCleanup, cleanuphandler.DeleteUser, "/api/v1/auth/users/x-1"},
		{autheps.DeleteGroupCleanup, cleanuphandler.DeleteGroup, "/api/v1/auth/groups/x-1"},
		{autheps.DeleteAccessRoleCleanup, cleanuphandler.DeleteAccessRole, "/api/v1/auth/accessroles/x-1"},
	} {
		t.Run(c.path, func(t *testing.T) {
			var forwarded []string
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				forwarded = append(forwarded, r.Header.Get(constants.HeaderUserID))
				_, _ = w.Write([]byte(`{"status":200}`))
			}))
			mux := router.NewRouter([]router.Route{router.CreateRoute(base.Delete, c.endpoint, c.handler)})
			r := httptest.NewRequest(http.MethodDelete, c.path, nil)
			r.Header.Set(constants.HeaderUserID, deleter)
			mux.ServeHTTP(httptest.NewRecorder(), r.WithContext(xauthz.WithIdentity(r.Context(), xauthz.Identity{UserID: deleter, Internal: true})))
			testutil.Equal(t, "forwarded callers", strings.Join(forwarded, ","), deleter)
		})
	}
}

// A session below the role's levels is refused before the exporter delete runs;
// auth deletes as an internal caller, so the exporter cannot cap it.
func TestDeleteRoleAndGroupCleanupAreCapped(t *testing.T) {
	cases := []struct {
		endpoint base.Endpoint
		handler  http.HandlerFunc
		path     string
	}{
		{autheps.DeleteGroupCleanup, cleanuphandler.DeleteGroup, "/api/v1/auth/groups/g-1"},
		{autheps.DeleteAccessRoleCleanup, cleanuphandler.DeleteAccessRole, "/api/v1/auth/accessroles/r-admin"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			var deletes int
			testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete {
					deletes++
				}
				data := `{"id":"r-admin","name":"Admin","scopesAndPermissions":[{"scope":"ALL","level":"Admin"}]}`
				if strings.Contains(r.URL.Path, "/groups/") {
					data = `{"id":"g-1","roleRefs":["r-admin"]}`
				}
				_, _ = w.Write([]byte(`{"data":` + data + `}`))
			}))
			mux := router.NewRouter([]router.Route{router.CreateRoute(base.Delete, c.endpoint, c.handler)})
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodDelete, c.path, nil)
			levels := map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelOwner}
			owner := xauthz.Identity{UserID: "u-owner", Grants: xauthz.Grants{Levels: levels}}
			mux.ServeHTTP(rec, r.WithContext(xauthz.WithIdentity(r.Context(), owner)))
			testutil.Equal(t, "status", rec.Code, http.StatusForbidden)
			testutil.Equal(t, "exporter deletes", deletes, 0)
		})
	}
}
