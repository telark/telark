package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"testing"

	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	bootstrapID = "u-boot"
	adminID     = "u-admin"
	ownerID     = "u-owner"
	plainID     = "u-plain"
	usersPath   = "/v1/users/"
	rolesPath   = "/v1/accessroles/"
)

var adminRoleID = constants.BuiltInRoleAdmin

// A directory of users and the built-in Admin role, served the way the exporter
// serves single-resource reads; unknown ids answer 404.
func stubDirectory(t *testing.T) {
	t.Helper()
	suspended := userresource.UserStatus{Phase: string(userresource.AccountPhaseSuspended)}
	users := map[string]*userresource.User{
		bootstrapID: {ID: bootstrapID, Bootstrap: true, RoleRefs: []*string{&adminRoleID}},
		adminID:     {ID: adminID, RoleRefs: []*string{&adminRoleID}, Status: suspended},
		ownerID:     {ID: ownerID},
		plainID:     {ID: plainID},
	}
	admin := roledata.AccessRole{
		ID: adminRoleID, Status: roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}},
	}
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload any
		switch {
		case strings.Contains(r.URL.Path, usersPath):
			id := r.URL.Path[strings.Index(r.URL.Path, usersPath)+len(usersPath):]
			user, ok := users[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			payload = user
		case strings.Contains(r.URL.Path, rolesPath):
			payload = admin
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": payload})
	}))
}

type caller struct {
	userID    string
	level     roledata.PermissionLevel
	internal  bool
	anonymous bool
	scope     string
}

func (c caller) ctx() context.Context {
	if c.anonymous {
		return context.Background()
	}
	scope := roledata.ScopeAll
	if c.scope != "" {
		scope = c.scope
	}
	grants := xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: c.level}}
	return xauthz.WithIdentity(context.Background(), xauthz.Identity{UserID: c.userID, Internal: c.internal, Grants: grants})
}

// Bootstrap users are never deleted through the API, any Admin on ALL may delete
// another administrator, and a caller below Admin is not told one exists. A
// suspended administrator is still one. Internal callers are not gated.
func TestGuardUserDelete(t *testing.T) {
	stubDirectory(t)
	owner := caller{userID: ownerID, level: roledata.PermissionLevelOwner}
	cases := []struct {
		name       string
		caller     caller
		target     string
		wantStatus int
	}{
		{"self", owner, ownerID, http.StatusForbidden},
		{"owner deletes plain user", owner, plainID, http.StatusOK},
		{"owner targets admin", owner, adminID, http.StatusNotFound},
		{"owner targets bootstrap", owner, bootstrapID, http.StatusNotFound},
		{"admin targets bootstrap", caller{userID: adminID, level: roledata.PermissionLevelAdmin}, bootstrapID, http.StatusForbidden},
		{"admin deletes admin", caller{userID: plainID, level: roledata.PermissionLevelAdmin}, adminID, http.StatusOK},
		{"bootstrap deletes admin", caller{userID: bootstrapID, level: roledata.PermissionLevelAdmin}, adminID, http.StatusOK},
		{"missing target", owner, "u-none", http.StatusNotFound},
		{"internal", caller{internal: true}, bootstrapID, http.StatusOK},
		{"no identity", caller{anonymous: true}, plainID, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := authz.GuardUserDelete(c.caller.ctx(), c.target)
			testutil.Equal(t, "status", status, c.wantStatus)
			testutil.Equal(t, "refused", err != nil, c.wantStatus != http.StatusOK)
		})
	}
}

const (
	appsOwnerRole = "r-apps-owner"
	appsReadRole  = "r-apps-read"
	downRole      = "r-down"
	missingID     = "x-none"
	appsGroup     = "g-apps"
	adminGroup    = "g-admin"
	downGroup     = "g-down"
)

// Roles and groups served by id the way the exporter serves single-resource
// reads; unknown ids answer 404 and the "down" role 500.
func stubRolesAndGroups(t *testing.T) {
	t.Helper()
	role := func(id, scope string, level roledata.PermissionLevel) roledata.AccessRole {
		return roledata.AccessRole{ID: id, Name: id, ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: scope, Level: level}}}
	}
	records := map[string]any{
		adminRoleID:   role(adminRoleID, roledata.ScopeAll, roledata.PermissionLevelAdmin),
		appsOwnerRole: role(appsOwnerRole, roledata.ScopeApplications, roledata.PermissionLevelOwner),
		appsReadRole:  role(appsReadRole, roledata.ScopeApplications, roledata.PermissionLevelReadOnly),
		appsGroup:     groupdata.Group{ID: appsGroup, RoleRefs: []string{appsReadRole, missingID}},
		adminGroup:    groupdata.Group{ID: adminGroup, RoleRefs: []string{appsReadRole, adminRoleID}},
		downGroup:     groupdata.Group{ID: downGroup, RoleRefs: []string{downRole}},
	}
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := path.Base(r.URL.Path)
		if id == downRole {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		record, ok := records[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": record})
	}))
}

// Deleting a role or group takes its levels from every holder, so each level
// must be within the caller's own on that scope or on ALL; Admin on ALL passes.
func TestGuardRoleAndGroupDelete(t *testing.T) {
	stubRolesAndGroups(t)
	admin := caller{userID: adminID, level: roledata.PermissionLevelAdmin}
	ownerOnAll := caller{userID: ownerID, level: roledata.PermissionLevelOwner}
	appsOwner := caller{userID: plainID, level: roledata.PermissionLevelOwner, scope: roledata.ScopeApplications}
	appsContributor := caller{userID: plainID, level: roledata.PermissionLevelContributor, scope: roledata.ScopeApplications}
	appsReader := caller{userID: plainID, level: roledata.PermissionLevelReadOnly, scope: roledata.ScopeApplications}
	internal := caller{internal: true}
	anonymous := caller{anonymous: true}
	cases := []struct {
		name       string
		guard      func(context.Context, string) (int, error)
		caller     caller
		target     string
		wantStatus int
	}{
		{"owner on ALL deletes the Admin role", authz.GuardRoleDelete, ownerOnAll, adminRoleID, http.StatusForbidden},
		{"admin deletes the Admin role", authz.GuardRoleDelete, admin, adminRoleID, http.StatusOK},
		{"apps owner deletes an apps owner role", authz.GuardRoleDelete, appsOwner, appsOwnerRole, http.StatusOK},
		{"apps contributor deletes an apps owner role", authz.GuardRoleDelete, appsContributor, appsOwnerRole, http.StatusForbidden},
		{"owner on ALL deletes an apps owner role", authz.GuardRoleDelete, ownerOnAll, appsOwnerRole, http.StatusOK},
		{"missing role", authz.GuardRoleDelete, ownerOnAll, missingID, http.StatusOK},
		{"role lookup down", authz.GuardRoleDelete, ownerOnAll, downRole, http.StatusServiceUnavailable},
		{"internal deletes the Admin role", authz.GuardRoleDelete, internal, adminRoleID, http.StatusOK},
		{"no identity", authz.GuardRoleDelete, anonymous, appsReadRole, http.StatusUnauthorized},
		{"owner on ALL deletes a group holding Admin", authz.GuardGroupDelete, ownerOnAll, adminGroup, http.StatusForbidden},
		{"admin deletes a group holding Admin", authz.GuardGroupDelete, admin, adminGroup, http.StatusOK},
		{"apps reader deletes a group with a missing role", authz.GuardGroupDelete, appsReader, appsGroup, http.StatusOK},
		{"missing group", authz.GuardGroupDelete, ownerOnAll, missingID, http.StatusOK},
		{"group role lookup down", authz.GuardGroupDelete, ownerOnAll, downGroup, http.StatusServiceUnavailable},
		{"internal deletes a group holding Admin", authz.GuardGroupDelete, internal, adminGroup, http.StatusOK},
		{"no identity on group", authz.GuardGroupDelete, anonymous, appsGroup, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := c.guard(c.caller.ctx(), c.target)
			testutil.Equal(t, "status", status, c.wantStatus)
			testutil.Equal(t, "refused", err != nil, c.wantStatus != http.StatusOK)
		})
	}
}
