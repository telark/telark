package writes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/services/exporter/internal/constants"
	roleutil "github.com/telark/telark/services/exporter/internal/utils/resources/role"
)

const (
	bareRule  = "rollbackapplication"
	validRule = "applications.rollbackapplication.deny"
)

func scopeWithRule(scope, rule string) []any {
	return []any{map[string]any{
		constants.FieldScope: scope, keyLevel: string(roledata.PermissionLevelContributor), constants.FieldRules: []any{rule},
	}}
}

func roleWithRule(scope, rule string) map[string]any {
	return map[string]any{
		keyName: appName, keyDesc: newDesc, constants.FieldCategoryRef: testCategory,
		constants.FieldScopesAndPermissions: scopeWithRule(scope, rule),
	}
}

// Seen live: a bare rule was stored, denied nothing and showed as its raw key.
func TestRoleWritesRefuseUnknownDenyRules(t *testing.T) {
	tests := []struct {
		name, scope, rule string
		known             bool
	}{
		{"bare rule", roledata.ScopeApplications, bareRule, false},
		{"unknown scope", "custom", "custom.rollbackapplication.deny", false},
		{"unknown rule", roledata.ScopeApplications, "applications.launchrockets.deny", false},
		{"rule of another scope", roledata.ScopeGroups, validRule, false},
		{"known rule", roledata.ScopeApplications, validRule, true},
		{"known rule under ALL", roledata.ScopeAll, validRule, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			installFake(t, storedRole(t, roledata.Protection{}))
			patch := map[string]any{constants.FieldScopesAndPermissions: scopeWithRule(tt.scope, tt.rule)}
			patched := patchAs(t, adminCaller(), rolesPath+pathSep+storedRoleID, jsonBody(t, patch))
			if tt.known {
				if patched.Code != http.StatusOK {
					t.Errorf("patch = %d %s, want 200", patched.Code, patched.Body.String())
				}
				// The fake API server cannot copy the create's finalizers, so the create is judged where the handler validates it.
				role, err := roleutil.ExtractRoleSpecFromRequestBody(roleWithRule(tt.scope, tt.rule))
				if err != nil || roleutil.ValidateAndPrepareRole(role, httptest.NewRecorder()) != nil {
					t.Errorf("create refused: %v", err)
				}
				return
			}
			created := serveAs(t, adminCaller(), http.MethodPost, rolesPath, jsonBody(t, roleWithRule(tt.scope, tt.rule)))
			for write, w := range map[string]*httptest.ResponseRecorder{"create": created, "patch": patched} {
				body := w.Body.String()
				if w.Code != http.StatusBadRequest || !strings.Contains(body, tt.rule) || !strings.Contains(body, validRule) {
					t.Errorf("%s = %d %q, want 400 naming the rule and the expected form", write, w.Code, body)
				}
			}
		})
	}
}

// Stored roles are not migrated: an edit that leaves the rules alone still saves.
func TestRolePatchKeepsStoredBareRules(t *testing.T) {
	bare := []string{bareRule}
	client := installFake(t, crSeedOf(t, v1alpha1.AccessRoleMetadata, storedRoleID, roledata.AccessRole{
		Name: appName, Description: newDesc, CategoryRef: testCategory, Type: roledata.RoleTypeCustom,
		Status: roledata.RoleStatusActive, Protection: &roledata.Protection{},
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{
			Scope: roledata.ScopeApplications, Level: roledata.PermissionLevelContributor, Rules: &bare,
		}},
	}))
	if w := patchAs(t, adminCaller(), rolesPath+pathSep+storedRoleID, jsonBody(t, map[string]any{keyDesc: newName})); w.Code != http.StatusOK {
		t.Fatalf("patch = %d %s", w.Code, w.Body.String())
	}
	if got := storedSpec(t, client, v1alpha1.AccessRoleMetadata, storedRoleID)[keyDesc]; got != newName {
		t.Errorf("description = %v, want %s", got, newName)
	}
}
