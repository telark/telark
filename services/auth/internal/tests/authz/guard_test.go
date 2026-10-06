package authz

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/telark/telark/services/auth/internal/helpers/shared"
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

const (
	appsAdminRole    = "r-apps-admin"
	appsAdminGroup   = "g-apps-admin"
	goneID           = "u-gone"
	suspendedID      = "u-suspended"
	appsAdminID      = "u-apps-admin"
	groupAppsAdminID = "u-group-apps-admin"
	enrolledID       = "u-enrolled"
)

func enrollDirectory() *testutil.FakeExporter {
	appsRole, groupRef := appsAdminRole, appsAdminGroup
	suspended := userresource.UserStatus{Phase: string(userresource.AccountPhaseSuspended)}
	role := func(id, scope string) *roledata.AccessRole {
		return &roledata.AccessRole{ID: id, Status: roledata.RoleStatusActive, ScopesAndPermissions: []roledata.ScopeAndPermissions{
			{Scope: scope, Level: roledata.PermissionLevelAdmin}}}
	}
	return &testutil.FakeExporter{
		Users: map[string]*userresource.User{
			bootstrapID:      {ID: bootstrapID, Bootstrap: true, RoleRefs: []*string{&adminRoleID}},
			adminID:          {ID: adminID, RoleRefs: []*string{&adminRoleID}},
			ownerID:          {ID: ownerID},
			plainID:          {ID: plainID},
			suspendedID:      {ID: suspendedID, Status: suspended},
			appsAdminID:      {ID: appsAdminID, RoleRefs: []*string{&appsRole}},
			groupAppsAdminID: {ID: groupAppsAdminID, GroupRefs: []*string{&groupRef}},
			enrolledID:       {ID: enrolledID},
		},
		Gone: map[string]bool{goneID: true},
		Roles: map[string]*roledata.AccessRole{
			adminRoleID:   role(adminRoleID, roledata.ScopeAll),
			appsAdminRole: role(appsAdminRole, roledata.ScopeApplications),
		},
		Groups:   map[string]*groupdata.Group{appsAdminGroup: {ID: appsAdminGroup, RoleRefs: []string{appsAdminRole}}},
		Passkeys: map[string]int{enrolledID: constants.DefaultIncrementValue},
	}
}

// An enroll link signs its holder in as the target, so issuing is capped like a role
// assignment; revoking skips only the refusals of a suspended or terminating account.
func TestGuardEnrollLink(t *testing.T) {
	testutil.StubBackend(t, enrollDirectory())
	usersOwner := caller{userID: ownerID, level: roledata.PermissionLevelOwner, scope: roledata.ScopeUsers}
	admin := caller{userID: adminID, level: roledata.PermissionLevelAdmin}
	bootstrapOwner := caller{userID: bootstrapID, level: roledata.PermissionLevelOwner, scope: roledata.ScopeUsers}
	cases := []struct {
		name          string
		caller        caller
		target        string
		issue, revoke int
	}{
		{"anonymous caller", caller{anonymous: true}, plainID, http.StatusUnauthorized, http.StatusUnauthorized},
		{"self", usersOwner, ownerID, http.StatusForbidden, http.StatusForbidden},
		{"missing target", usersOwner, missingID, http.StatusNotFound, http.StatusNotFound},
		{"admin target, restricted caller", usersOwner, adminID, http.StatusNotFound, http.StatusNotFound},
		{"bootstrap target, restricted caller", usersOwner, bootstrapID, http.StatusNotFound, http.StatusNotFound},
		{"bootstrap target, admin caller", admin, bootstrapID, http.StatusForbidden, http.StatusForbidden},
		{"terminating target", usersOwner, goneID, http.StatusGone, http.StatusOK},
		{"suspended target", usersOwner, suspendedID, http.StatusConflict, http.StatusOK},
		{"plain target", usersOwner, plainID, http.StatusOK, http.StatusOK},
		{"target above the caller", usersOwner, appsAdminID, http.StatusForbidden, http.StatusForbidden},
		{"target above the caller through a group", usersOwner, groupAppsAdminID, http.StatusForbidden, http.StatusForbidden},
		{"admin caller over a group-granted target", admin, groupAppsAdminID, http.StatusOK, http.StatusOK},
		{"enrolled target, users owner", usersOwner, enrolledID, http.StatusForbidden, http.StatusForbidden},
		{"enrolled target, admin on ALL", admin, enrolledID, http.StatusOK, http.StatusOK},
		{"enrolled target, bootstrap account", bootstrapOwner, enrolledID, http.StatusOK, http.StatusOK},
		{"service token", caller{internal: true}, bootstrapID, http.StatusOK, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := authz.GuardEnrollLinkIssue(c.caller.ctx(), c.target)
			testutil.Equal(t, "issue", status, c.issue)
			testutil.Equal(t, "issue refused", err != nil, c.issue != http.StatusOK)
			status, err = authz.GuardEnrollLinkRevoke(c.caller.ctx(), c.target)
			testutil.Equal(t, "revoke", status, c.revoke)
			testutil.Equal(t, "revoke refused", err != nil, c.revoke != http.StatusOK)
		})
	}
}

// Sign-in settings take the bootstrap account, read from the caller's own record rather
// than its grants; a record that cannot be read is an outage, not a refusal.
func TestGuardBootstrapCaller(t *testing.T) {
	cases := []struct {
		name   string
		caller caller
		down   bool
		status int
	}{
		{"bootstrap account", caller{userID: bootstrapID, level: roledata.PermissionLevelAdmin}, false, http.StatusOK},
		{"admin on ALL", caller{userID: adminID, level: roledata.PermissionLevelAdmin}, false, http.StatusForbidden},
		{"service token", caller{internal: true}, false, http.StatusOK},
		{"anonymous caller", caller{anonymous: true}, false, http.StatusUnauthorized},
		{"caller record unreadable", caller{userID: bootstrapID, level: roledata.PermissionLevelAdmin}, true, http.StatusServiceUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := enrollDirectory()
			testutil.StubBackend(t, fake)
			fake.SetDown(c.down)
			status, err := authz.GuardBootstrapCaller(c.caller.ctx())
			testutil.Equal(t, "status", status, c.status)
			testutil.Equal(t, "refused", err != nil, c.status != http.StatusOK)
		})
	}
}

// Every refusal carries a code the UI can branch on instead of the wording, which may change.
func TestGuardRefusalCodes(t *testing.T) {
	testutil.StubBackend(t, enrollDirectory())
	usersOwner := caller{userID: ownerID, level: roledata.PermissionLevelOwner, scope: roledata.ScopeUsers}
	ownerOnAll := caller{userID: ownerID, level: roledata.PermissionLevelOwner}
	admin := caller{userID: adminID, level: roledata.PermissionLevelAdmin}
	cases := []struct {
		name  string
		guard func() (int, error)
		code  string
	}{
		{"own account deleted", func() (int, error) {
			return authz.GuardUserDelete(usersOwner.ctx(), ownerID)
		}, "self_delete"},
		{"bootstrap account deleted", func() (int, error) {
			return authz.GuardUserDelete(admin.ctx(), bootstrapID)
		}, "bootstrap_managed"},
		{"role above the caller deleted", func() (int, error) {
			return authz.GuardRoleDelete(ownerOnAll.ctx(), adminRoleID)
		}, "role_above_caller_level"},
		{"link for oneself", func() (int, error) {
			return authz.GuardEnrollLinkIssue(usersOwner.ctx(), ownerID)
		}, "enroll_link_self"},
		{"link for the bootstrap account", func() (int, error) {
			return authz.GuardEnrollLinkIssue(admin.ctx(), bootstrapID)
		}, "enroll_link_bootstrap"},
		{"link above the caller", func() (int, error) {
			return authz.GuardEnrollLinkIssue(usersOwner.ctx(), appsAdminID)
		}, "enroll_link_above_level"},
		{"recovery link", func() (int, error) {
			return authz.GuardEnrollLinkIssue(usersOwner.ctx(), enrolledID)
		}, "enroll_link_recovery"},
		{"sign-in settings", func() (int, error) {
			return authz.GuardBootstrapCaller(admin.ctx())
		}, "sign_in_settings_bootstrap_only"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, err := c.guard()
			testutil.Equal(t, "status", status, http.StatusForbidden)
			refusal, ok := errors.AsType[*shared.RefusalError](err)
			if !ok {
				t.Fatalf("refusal %v carries no code", err)
			}
			testutil.Equal(t, "code", refusal.Code, c.code)
		})
	}
}
