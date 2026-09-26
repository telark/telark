package authz

import (
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	statusActive    = "Active"
	preventDeletion = "preventDeletion"
)

// Reactivating or extending a role above the caller puts its levels back into
// effect, exactly like writing them; a body without those fields is not capped.
func TestGuardPatchedRoleLevels(t *testing.T) {
	rolesContributor := levels(roledata.ScopeRoles, roledata.PermissionLevelContributor)
	above := &roledata.RoleAsResource{ScopesAndPermissions: []roledata.ScopeAndPermissions{
		{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin},
	}}
	within := &roledata.RoleAsResource{ScopesAndPermissions: []roledata.ScopeAndPermissions{
		{Scope: roledata.ScopeRoles, Level: roledata.PermissionLevelReadOnly},
	}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		merged   *roledata.RoleAsResource
		body     map[string]any
		want     bool
	}{
		{"reactivate above caller", rolesContributor, above, map[string]any{constants.FieldStatus: statusActive}, false},
		{"extend validity above caller", rolesContributor, above,
			map[string]any{constants.FieldValidity: map[string]any{"type": "permanent"}}, false},
		{"rename above caller", rolesContributor, above, map[string]any{constants.FieldDescription: "d"}, true},
		{"reactivate within caller", rolesContributor, within, map[string]any{constants.FieldStatus: statusActive}, true},
		{"internal reactivates", internalIdentity, above, map[string]any{constants.FieldStatus: statusActive}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if got := authz.GuardPatchedRoleLevels(w, requestAs(tt.identity), tt.merged, tt.body); got != tt.want {
				t.Fatalf("GuardPatchedRoleLevels = %v, want %v (%d)", got, tt.want, w.Code)
			}
		})
	}
}

// A built-in type or a protection flag from a session would mint a role nobody
// can edit or delete.
func TestGuardRoleReservedFields(t *testing.T) {
	admin := levels(roledata.ScopeAll, roledata.PermissionLevelAdmin)
	protected := &roledata.RoleAsResource{Protection: &roledata.Protection{PreventDeletion: true}}
	tests := []struct {
		name     string
		identity xauthz.Identity
		existing *roledata.RoleAsResource
		body     map[string]any
		want     bool
	}{
		{"create built-in", admin, nil, map[string]any{constants.FieldType: string(roledata.RoleTypeBuiltIn)}, false},
		{"create custom", admin, nil, map[string]any{constants.FieldType: string(roledata.RoleTypeCustom)}, true},
		{"create protected", admin, nil, map[string]any{constants.FieldProtection: map[string]any{preventDeletion: true}}, false},
		{"create with all flags off", admin, nil, map[string]any{constants.FieldProtection: map[string]any{preventDeletion: false}}, true},
		{"patch drops protection", admin, protected, map[string]any{constants.FieldProtection: map[string]any{}}, false},
		{"patch echoes protection", admin, protected, map[string]any{constants.FieldProtection: map[string]any{preventDeletion: true}}, true},
		{"patch to built-in", admin, &roledata.RoleAsResource{}, map[string]any{constants.FieldType: string(roledata.RoleTypeBuiltIn)}, false},
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
