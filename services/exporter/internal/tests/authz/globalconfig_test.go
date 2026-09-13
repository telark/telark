package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	globalconfigresource "github.com/telark/data/resources/globalconfig"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	xauthz "github.com/telark/x-ware/authz"
)

func settingsIdentity(level roledata.PermissionLevel, denied ...string) xauthz.Identity {
	grants := xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeSettings: level},
	}
	if len(denied) > 0 {
		grants.Denied = map[string][]string{roledata.ScopeSettings: denied}
	}
	return xauthz.Identity{UserID: callerID, Grants: grants}
}

func patchRequest(identity xauthz.Identity) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, "/v1/resources/globalconfig/patch", nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

// The route's own check is Contributor, so without this guard a Contributor
// could write every part of the config, including the AI keys the role model
// reserves for Owner.
func TestGuardGlobalConfigPatchLevelPerField(t *testing.T) {
	tests := []struct {
		name  string
		level roledata.PermissionLevel
		field string
		want  bool
	}{
		{"contributor edits discovery config", roledata.PermissionLevelContributor, globalconfigresource.FieldExcludedNamespaces, true},
		{"contributor edits snapshot storage", roledata.PermissionLevelContributor, globalconfigresource.FieldSnapshots, true},
		{"contributor cannot touch ai", roledata.PermissionLevelContributor, globalconfigresource.FieldAI, false},
		{"owner edits ai", roledata.PermissionLevelOwner, globalconfigresource.FieldAI, true},
		{"owner cannot touch oidc", roledata.PermissionLevelOwner, globalconfigresource.FieldOIDC, false},
		{"admin edits oidc", roledata.PermissionLevelAdmin, globalconfigresource.FieldOIDC, true},
		{"readonly edits nothing", roledata.PermissionLevelReadOnly, globalconfigresource.FieldExcludedNamespaces, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			spec := map[string]any{tt.field: "value"}

			got := authz.GuardGlobalConfigPatch(w, patchRequest(settingsIdentity(tt.level)), spec)

			if got != tt.want {
				t.Errorf("GuardGlobalConfigPatch() = %v, want %v", got, tt.want)
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

// The three settings rules an admin can untick on a role must actually bite.
func TestGuardGlobalConfigPatchHonoursDenyRules(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		action string
		level  roledata.PermissionLevel
	}{
		{"discovery config denied", globalconfigresource.FieldExcludedNamespaces, roledata.ActionEditDiscoveryConfig, roledata.PermissionLevelOwner},
		{"snapshot storage denied", globalconfigresource.FieldSnapshots, roledata.ActionEditSnapshotStorage, roledata.PermissionLevelOwner},
		{"ai insights denied", globalconfigresource.FieldAI, roledata.ActionControlAIInsights, roledata.PermissionLevelOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rule := xauthz.RuleKey(roledata.ScopeSettings, tt.action)
			identity := settingsIdentity(tt.level, rule)

			if authz.GuardGlobalConfigPatch(w, patchRequest(identity), map[string]any{tt.field: "value"}) {
				t.Errorf("deny rule %q did not block the field", rule)
			}
		})
	}
}

// Denying one setting must not withhold the others.
func TestGuardGlobalConfigPatchDenyIsPerField(t *testing.T) {
	w := httptest.NewRecorder()
	rule := xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditSnapshotStorage)
	identity := settingsIdentity(roledata.PermissionLevelOwner, rule)

	spec := map[string]any{globalconfigresource.FieldExcludedNamespaces: []any{"kube-system"}}
	if !authz.GuardGlobalConfigPatch(w, patchRequest(identity), spec) {
		t.Error("a deny on snapshot storage blocked discovery config")
	}
}

// A body touching several parts must satisfy the strictest of them.
func TestGuardGlobalConfigPatchChecksEveryFieldPresent(t *testing.T) {
	w := httptest.NewRecorder()
	spec := map[string]any{
		globalconfigresource.FieldExcludedNamespaces: []any{"kube-system"},
		globalconfigresource.FieldAI:                 map[string]any{"enabled": true},
	}

	identity := settingsIdentity(roledata.PermissionLevelContributor)
	if authz.GuardGlobalConfigPatch(w, patchRequest(identity), spec) {
		t.Error("an Owner-only field slipped through beside a Contributor one")
	}
}

// The reported cluster version is not a privilege, so a user holding no scope
// at all must still be able to set it.
func TestGuardGlobalConfigPatchLeavesUngovernedFieldsOpen(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]any
	}{
		{"cluster version", map[string]any{globalconfigresource.FieldCluster: map[string]any{"version": "1.31"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			noScopes := xauthz.Identity{UserID: callerID}

			if !authz.GuardGlobalConfigPatch(w, patchRequest(noScopes), tt.spec) {
				t.Error("a user with no scopes was refused a setting that is not a privilege")
			}
		})
	}
}

// The key now lives in a Secret and is added to the response only when this
// gate allows it, so the gate is what keeps it from ReadOnly users.
func TestMayControlAIInsightsDeniesBelowOwner(t *testing.T) {
	levels := []roledata.PermissionLevel{
		roledata.PermissionLevelReadOnly,
		roledata.PermissionLevelContributor,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			if authz.MayControlAIInsights(patchRequest(settingsIdentity(level))) {
				t.Errorf("%s user would receive the provider API key", level)
			}
		})
	}
}

func TestMayControlAIInsightsAllowsOwner(t *testing.T) {
	if !authz.MayControlAIInsights(patchRequest(settingsIdentity(roledata.PermissionLevelOwner))) {
		t.Error("owner cannot read the key they are allowed to change")
	}
}

// Withholding the AI action must withhold the key it protects.
func TestMayControlAIInsightsHonoursDenyRule(t *testing.T) {
	rule := xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionControlAIInsights)

	if authz.MayControlAIInsights(patchRequest(settingsIdentity(roledata.PermissionLevelAdmin, rule))) {
		t.Error("a user denied AI insights would receive the key")
	}
}

func TestMayControlAIInsightsDeniesWithoutIdentity(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/v1/resources/globalconfig/get", nil)

	if authz.MayControlAIInsights(r) {
		t.Error("a request carrying no identity would receive the key")
	}
}

func TestGuardGlobalConfigPatchDeniesWithoutIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/v1/resources/globalconfig/patch", nil)

	if authz.GuardGlobalConfigPatch(w, r, map[string]any{globalconfigresource.FieldAI: "x"}) {
		t.Error("allowed with no identity in context")
	}
}

func TestGuardGlobalConfigPatchAllowsInternalCaller(t *testing.T) {
	w := httptest.NewRecorder()
	identity := xauthz.Identity{Internal: true}

	spec := map[string]any{globalconfigresource.FieldCluster: map[string]any{"version": "1.31"}}
	if !authz.GuardGlobalConfigPatch(w, patchRequest(identity), spec) {
		t.Error("internal caller was blocked")
	}
}
