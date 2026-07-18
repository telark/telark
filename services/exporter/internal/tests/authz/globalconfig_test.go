package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	globalconfigresource "github.com/telark/data/resources/globalconfig"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	xauthz "github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

// Display preferences and the reported cluster version are not privileges. A
// user holding no scope at all must still be able to set their own theme.
func TestGuardGlobalConfigPatchLeavesUngovernedFieldsOpen(t *testing.T) {
	tests := []struct {
		name string
		spec map[string]any
	}{
		{"display preferences", map[string]any{globalconfigresource.FieldUserSettings: map[string]any{"theme": "dark"}}},
		{"cluster version", map[string]any{globalconfigresource.FieldCluster: map[string]any{"version": "1.31"}}},
		{
			name: "both together",
			spec: map[string]any{
				globalconfigresource.FieldUserSettings: map[string]any{"density": "compact"},
				globalconfigresource.FieldCluster:      map[string]any{"version": "1.31"},
			},
		},
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

func configResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			globalconfigresource.FieldAI: map[string]any{
				"enabled":             true,
				"provider":            "gemini",
				globalconfigresource.FieldAPIKey: "sk-secret-value",
			},
			globalconfigresource.FieldUserSettings: map[string]any{"theme": "dark"},
		},
	}}
}

func apiKeyIn(resource *unstructured.Unstructured) (string, bool) {
	spec, _ := resource.Object["spec"].(map[string]any)
	ai, _ := spec[globalconfigresource.FieldAI].(map[string]any)
	key, present := ai[globalconfigresource.FieldAPIKey]
	value, _ := key.(string)
	return value, present
}

// The key is part of AI insights, so seeing it takes the same right as
// changing it. Every role can read the config, so without this the secret
// reaches ReadOnly users.
func TestRedactGlobalConfigHidesAPIKeyBelowOwner(t *testing.T) {
	levels := []roledata.PermissionLevel{
		roledata.PermissionLevelReadOnly,
		roledata.PermissionLevelContributor,
	}

	for _, level := range levels {
		t.Run(string(level), func(t *testing.T) {
			resource := configResource()

			authz.RedactGlobalConfig(patchRequest(settingsIdentity(level)), resource)

			if _, present := apiKeyIn(resource); present {
				t.Errorf("%s user received the provider API key", level)
			}
		})
	}
}

func TestRedactGlobalConfigKeepsAPIKeyForOwner(t *testing.T) {
	resource := configResource()

	authz.RedactGlobalConfig(patchRequest(settingsIdentity(roledata.PermissionLevelOwner)), resource)

	value, present := apiKeyIn(resource)
	if !present || value != "sk-secret-value" {
		t.Error("owner could not read the key they are allowed to change")
	}
}

// Withholding the AI action must withhold the key it protects.
func TestRedactGlobalConfigHonoursDenyRule(t *testing.T) {
	rule := xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionControlAIInsights)
	resource := configResource()

	authz.RedactGlobalConfig(patchRequest(settingsIdentity(roledata.PermissionLevelAdmin, rule)), resource)

	if _, present := apiKeyIn(resource); present {
		t.Error("a user denied AI insights received the key")
	}
}

func TestRedactGlobalConfigWithoutIdentityHidesKey(t *testing.T) {
	resource := configResource()
	r := httptest.NewRequest(http.MethodGet, "/v1/resources/globalconfig/get", nil)

	authz.RedactGlobalConfig(r, resource)

	if _, present := apiKeyIn(resource); present {
		t.Error("the key survived a request carrying no identity")
	}
}

// Redaction must not disturb the rest of the config.
func TestRedactGlobalConfigLeavesOtherFields(t *testing.T) {
	resource := configResource()

	authz.RedactGlobalConfig(patchRequest(settingsIdentity(roledata.PermissionLevelReadOnly)), resource)

	spec, _ := resource.Object["spec"].(map[string]any)
	ai, _ := spec[globalconfigresource.FieldAI].(map[string]any)
	if ai["provider"] != "gemini" || ai["enabled"] != true {
		t.Error("redaction removed more than the key")
	}
	if _, ok := spec[globalconfigresource.FieldUserSettings]; !ok {
		t.Error("redaction dropped unrelated settings")
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
