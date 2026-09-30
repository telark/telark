package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	roleA = "r-00000-0000-000a"
	roleB = "r-00000-0000-000b"
)

var internalIdentity = xauthz.Identity{Internal: true}

func expectForbidden(t *testing.T, w *httptest.ResponseRecorder, allowed bool, what string) {
	t.Helper()
	if allowed {
		t.Fatalf("%s was allowed", what)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("%s: got %d, want 403", what, w.Code)
	}
}

// The escalation seen live: a rollback-denied user forged spec.rollbacks and
// the controller executed it.
func TestGuardApplicationPatchSessionLimitedToUserFields(t *testing.T) {
	owner := levels(roledata.ScopeApplications, roledata.PermissionLevelOwner)
	forged := map[string]map[string]any{
		"rollbacks": {constants.FieldRollbacks: []any{map[string]any{"status": "pending"}}},
		"name":      {constants.FieldName: "renamed"},
		"health":    {"health": map[string]any{"status": "healthy"}},
		"snapshots": {constants.FieldSnapshots: []any{}},
		"history":   {constants.FieldHistory: map[string]any{}},
		"mixed":     {constants.FieldDisplayName: "ok", constants.FieldResources: []any{}},
	}
	for key, target := range forged {
		w := httptest.NewRecorder()
		expectForbidden(t, w, authz.GuardApplicationPatch(w, requestAs(owner), target, target), key)
	}

	edit := map[string]any{constants.FieldDisplayName: "Shop", constants.FieldDescription: "storefront"}
	w := httptest.NewRecorder()
	beside := map[string]any{constants.SpecField: edit, constants.MetadataField: map[string]any{"finalizers": []any{}}}
	expectForbidden(t, w, authz.GuardApplicationPatch(w, requestAs(owner), beside, edit), "metadata beside spec")

	w = httptest.NewRecorder()
	wrapped := map[string]any{constants.SpecField: edit}
	if !authz.GuardApplicationPatch(w, requestAs(owner), wrapped, edit) {
		t.Fatalf("UI edit denied: %d %s", w.Code, w.Body.String())
	}
	if !authz.GuardApplicationPatch(w, requestAs(internalIdentity), forged["rollbacks"], forged["rollbacks"]) {
		t.Fatal("internal caller denied")
	}
}

func TestGuardUserCreatePrivilegedFields(t *testing.T) {
	contributor := userWithLevel(roledata.PermissionLevelContributor)
	plain := map[string]any{
		constants.FieldUsername:  "testuser",
		constants.FieldRoleRefs:  []any{},
		constants.FieldGroupRefs: []any{},
		constants.FieldStatus:    constants.EmptyString,
	}
	w := httptest.NewRecorder()
	if !authz.GuardUserCreate(w, requestAs(contributor), plain) {
		t.Fatalf("plain create denied: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	expectForbidden(t, w, authz.GuardUserCreate(w, requestAs(contributor), rolePromotion()), "contributor creating with roles")

	w = httptest.NewRecorder()
	if !authz.GuardUserCreate(w, requestAs(userWithLevel(roledata.PermissionLevelOwner)), rolePromotion()) {
		t.Fatalf("owner create with roles denied: %d %s", w.Code, w.Body.String())
	}
	if !authz.GuardUserCreate(w, requestAs(internalIdentity), rolePromotion()) {
		t.Fatal("internal caller denied")
	}
}

func TestGuardGroupRolesPatch(t *testing.T) {
	existing := []string{roleA}
	attach := map[string]any{constants.FieldRoleRefs: []any{roleA, roleB}}
	remove := map[string]any{constants.FieldRoleRefs: []any{}}
	unchanged := map[string]any{constants.FieldRoleRefs: []any{roleA}, constants.FieldName: "team"}

	tests := []struct {
		name     string
		identity xauthz.Identity
		body     map[string]any
		want     bool
	}{
		{"contributor attaches", levels(roledata.ScopeGroups, roledata.PermissionLevelContributor), attach, false},
		{"owner attaches", levels(roledata.ScopeGroups, roledata.PermissionLevelOwner), attach, true},
		{"owner denied attach", denied(levels(roledata.ScopeGroups, roledata.PermissionLevelOwner), roledata.ScopeGroups,
			xauthz.RuleKey(roledata.ScopeGroups, roledata.ActionAttachRoleToGroup)), attach, false},
		{"owner denied remove", denied(levels(roledata.ScopeGroups, roledata.PermissionLevelOwner), roledata.ScopeGroups,
			xauthz.RuleKey(roledata.ScopeGroups, roledata.ActionRemoveRoleFromGroup)), remove, false},
		{"denied attach may still remove", denied(levels(roledata.ScopeGroups, roledata.PermissionLevelOwner), roledata.ScopeGroups,
			xauthz.RuleKey(roledata.ScopeGroups, roledata.ActionAttachRoleToGroup)), remove, true},
		{"contributor unchanged roles", levels(roledata.ScopeGroups, roledata.PermissionLevelContributor), unchanged, true},
		{"contributor no roles key", levels(roledata.ScopeGroups, roledata.PermissionLevelContributor),
			map[string]any{constants.FieldName: "x"}, true},
		{"internal", internalIdentity, attach, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardGroupRolesPatch(w, requestAs(tt.identity), existing, tt.body)
			if got != tt.want {
				t.Fatalf("GuardGroupRolesPatch = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGuardRoleLevelsCapsAtCallerLevel(t *testing.T) {
	appsOwner := []roledata.ScopeAndPermissions{{Scope: roledata.ScopeApplications, Level: roledata.PermissionLevelOwner}}
	allAdmin := []roledata.ScopeAndPermissions{{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}}

	tests := []struct {
		name     string
		identity xauthz.Identity
		scopes   []roledata.ScopeAndPermissions
		want     bool
	}{
		{"roles contributor grants apps owner", levels(roledata.ScopeRoles, roledata.PermissionLevelContributor), appsOwner, false},
		{"apps contributor grants apps owner", levels(roledata.ScopeApplications, roledata.PermissionLevelContributor), appsOwner, false},
		{"apps owner grants apps owner", levels(roledata.ScopeApplications, roledata.PermissionLevelOwner), appsOwner, true},
		{"all owner grants apps owner", levels(roledata.ScopeAll, roledata.PermissionLevelOwner), appsOwner, true},
		{"all owner grants all admin", levels(roledata.ScopeAll, roledata.PermissionLevelOwner), allAdmin, false},
		{"all admin grants all admin", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), allAdmin, true},
		{"apps admin grants all admin", levels(roledata.ScopeApplications, roledata.PermissionLevelAdmin), allAdmin, false},
		{"unknown level never granted", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin),
			[]roledata.ScopeAndPermissions{{Scope: roledata.ScopeUsers, Level: "Root"}}, false},
		{"internal", internalIdentity, allAdmin, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := authz.GuardRoleLevels(w, requestAs(tt.identity), tt.scopes)
			if got != tt.want {
				t.Fatalf("GuardRoleLevels = %v, want %v (%d %s)", got, tt.want, w.Code, w.Body.String())
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}
