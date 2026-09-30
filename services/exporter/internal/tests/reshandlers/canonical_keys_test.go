package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	protectionhandler "github.com/telark/telark/services/exporter/internal/handlers/plans/protection"
	grouphandler "github.com/telark/telark/services/exporter/internal/handlers/resources/group"
	rolehandler "github.com/telark/telark/services/exporter/internal/handlers/resources/role"
	userhandler "github.com/telark/telark/services/exporter/internal/handlers/resources/user"
)

const keyRejected = "is not recognized"

// encoding/json would accept these case variants while the guards only read the
// exact keys, so a create carrying them used to persist privileges unchecked.
func TestCreateRejectsCaseVariantPrivilegedKeys(t *testing.T) {
	o := newOptimizer(t)
	contributor := xauthz.Identity{UserID: testUserID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelContributor},
	}}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"group roles", grouphandler.CreateGroupResourceWithCacheInvalidation(o),
			`{"name":"g","description":"d","categoryRef":"c","RoleRefs":["role-admin"]}`},
		{"group members", grouphandler.CreateGroupResourceWithCacheInvalidation(o),
			`{"name":"g","description":"d","categoryRef":"c","ASSIGNEDUSERSIDS":["u1"]}`},
		{"user roles", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"x@example.com","RoleRefs":["role-admin"]}`},
		{"user bootstrap", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"x@example.com","Bootstrap":true}`},
		{"user identities", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"x@example.com","Identities":[{"provider":"google","issuer":"i","subject":"s"}]}`},
		{"user nested status", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"x@example.com","status":{"Phase":"active"}}`},
		{"role protection", rolehandler.CreateRoleResourceWithCacheInvalidation(o),
			`{"name":"r","Protection":{"preventDeletion":true}}`},
		{"plan phase", protectionhandler.CreatePlan(), `{"id":"p","Phase":"active","Mode":"enforce"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := varsReq(http.MethodPost, tt.body)
			r = r.WithContext(xauthz.WithIdentity(r.Context(), contributor))
			rec := httptest.NewRecorder()
			tt.handler(rec, r)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), keyRejected) {
				t.Fatalf("code = %d body %q, want 400 naming the key", rec.Code, rec.Body.String())
			}
		})
	}
}

// A null or empty id used to be dropped silently, creating the account with less than was asked.
func TestCreateRejectsInvalidRefIDs(t *testing.T) {
	o := newOptimizer(t)
	admin := xauthz.Identity{UserID: testUserID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
	tests := []struct {
		name    string
		handler http.HandlerFunc
		body    string
	}{
		{"user null role", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"test@example.com","roleRefs":[null]}`},
		{"user empty group", userhandler.CreateUserResourceWithCacheInvalidation(o),
			`{"username":"x","email":"test@example.com","groupRefs":[""]}`},
		{"group null member", grouphandler.CreateGroupResourceWithCacheInvalidation(o),
			`{"name":"g","description":"d","categoryRef":"c","userRefs":[null]}`},
		{"group numeric role", grouphandler.CreateGroupResourceWithCacheInvalidation(o),
			`{"name":"g","description":"d","categoryRef":"c","roleRefs":[1]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := varsReq(http.MethodPost, tt.body)
			r = r.WithContext(xauthz.WithIdentity(r.Context(), admin))
			rec := httptest.NewRecorder()
			tt.handler(rec, r)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "every id must be a non-empty string") {
				t.Fatalf("code = %d body %q, want 400 naming the refs", rec.Code, rec.Body.String())
			}
		})
	}
}
