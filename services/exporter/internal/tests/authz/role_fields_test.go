package authz

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	statusActive    = "Active"
	preventDeletion = "preventDeletion"
)

// Reactivating or extending a role above the caller puts its levels back into
// effect, exactly like writing them, and editing the levels of a role above the
// caller takes them from its holders; a body without those fields is not capped.
func TestGuardPatchedRoleLevels(t *testing.T) {
	rolesContributor := levels(roledata.ScopeRoles, roledata.PermissionLevelContributor)
	above := &roledata.AccessRole{ScopesAndPermissions: []roledata.ScopeAndPermissions{
		{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin},
	}}
	within := &roledata.AccessRole{ScopesAndPermissions: []roledata.ScopeAndPermissions{
		{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly},
	}}
	demote := map[string]any{constants.FieldScopesAndPermissions: []any{}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		existing *roledata.AccessRole
		merged   *roledata.AccessRole
		body     map[string]any
		want     bool
	}{
		{"reactivate above caller", rolesContributor, above, above, map[string]any{constants.FieldStatus: statusActive}, false},
		{"extend validity above caller", rolesContributor, above, above,
			map[string]any{constants.FieldValidity: map[string]any{"type": "permanent"}}, false},
		{"rename above caller", rolesContributor, above, above, map[string]any{constants.FieldDescription: "d"}, true},
		{"reactivate within caller", rolesContributor, within, within, map[string]any{constants.FieldStatus: statusActive}, true},
		{"demote a role above caller", rolesContributor, above, within, demote, false},
		{"deactivate a role above caller", rolesContributor, above, above,
			map[string]any{constants.FieldStatus: string(roledata.RoleStatusInactive)}, false},
		{"admin on ALL demotes", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), above, within, demote, true},
		{"internal reactivates", internalIdentity, above, above, map[string]any{constants.FieldStatus: statusActive}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardPatchedRoleLevels(w, requestAs(tt.identity), tt.existing, tt.merged, tt.body); got != tt.want {
				t.Fatalf("GuardPatchedRoleLevels = %v, want %v (%d)", got, tt.want, w.Code)
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}

// Deleting a role takes its levels from every holder, so only a caller holding
// them may; seen live: a roles Owner deleted an ALL: Admin custom role.
func TestGuardRoleWithinCaller(t *testing.T) {
	allAdmin := roleGranting("all-admin", roledata.ScopeAll, roledata.PermissionLevelAdmin)
	tests := []struct {
		name     string
		identity xauthz.Identity
		role     *roledata.AccessRole
		want     bool
	}{
		{"roles owner deletes ALL admin", levels(roledata.ScopeRoles, roledata.PermissionLevelOwner), allAdmin, false},
		{"roles owner deletes roles owner", levels(roledata.ScopeRoles, roledata.PermissionLevelOwner),
			roleGranting("roles-owner", roledata.ScopeRoles, roledata.PermissionLevelOwner), true},
		{"admin on ALL deletes ALL admin", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), allAdmin, true},
		{"internal deletes ALL admin", internalIdentity, allAdmin, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardRoleWithinCaller(w, requestAs(tt.identity), tt.role); got != tt.want {
				t.Fatalf("GuardRoleWithinCaller = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && (w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), tt.role.Name)) {
				t.Fatalf("denial = %d %s, want 403 naming %q", w.Code, w.Body.String(), tt.role.Name)
			}
		})
	}
}

// A session may never mint a built-in role; a custom role's protection is set
// by whoever creates it and changed only by its creator or an Admin on ALL.
func TestGuardRoleReservedFields(t *testing.T) {
	admin := levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)
	rolesOwner := levels(roledata.ScopeRoles, roledata.PermissionLevelOwner)
	creator, other := rolesOwner.UserID, "u-other"
	protectedBy := func(author string) *roledata.AccessRole {
		return &roledata.AccessRole{
			Type: roledata.RoleTypeCustom, CreatedBy: &author, Protection: &roledata.Protection{PreventDeletion: true},
		}
	}
	builtIn := &roledata.AccessRole{Type: roledata.RoleTypeBuiltIn, Protection: &roledata.Protection{PreventDeletion: true}}
	unlock := map[string]any{constants.FieldProtection: map[string]any{}}
	lock := map[string]any{constants.FieldProtection: map[string]any{preventDeletion: true}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		existing *roledata.AccessRole
		body     map[string]any
		want     bool
	}{
		{"create built-in", admin, nil, map[string]any{constants.FieldType: string(roledata.RoleTypeBuiltIn)}, false},
		{"create custom", admin, nil, map[string]any{constants.FieldType: string(roledata.RoleTypeCustom)}, true},
		{"creator sets protection on create", rolesOwner, nil, lock, true},
		{"create with all flags off", admin, nil, map[string]any{constants.FieldProtection: map[string]any{preventDeletion: false}}, true},
		{"admin on ALL unlocks a custom role", admin, protectedBy(other), unlock, true},
		{"creator unlocks their role", rolesOwner, protectedBy(creator), unlock, true},
		{"non-creator unlocks", rolesOwner, protectedBy(other), unlock, false},
		{"non-creator nulls protection", rolesOwner, protectedBy(other), map[string]any{constants.FieldProtection: nil}, false},
		{"null protection on an unprotected role", rolesOwner, &roledata.AccessRole{}, map[string]any{constants.FieldProtection: nil}, true},
		{"non-creator echoes protection", rolesOwner, protectedBy(other), lock, true},
		{"admin on ALL unlocks a built-in role", admin, builtIn, unlock, false},
		{"patch to built-in", admin, &roledata.AccessRole{}, map[string]any{constants.FieldType: string(roledata.RoleTypeBuiltIn)}, false},
		{"internal seeds built-in", internalIdentity, nil, map[string]any{
			constants.FieldType: string(roledata.RoleTypeBuiltIn), constants.FieldProtection: map[string]any{preventDeletion: true},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardRoleReservedFields(w, requestAs(tt.identity), tt.existing, tt.body); got != tt.want {
				t.Fatalf("GuardRoleReservedFields = %v, want %v (%d)", got, tt.want, w.Code)
			}
		})
	}
}
