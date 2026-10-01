package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const keyEnabled = "enabled"

func settingsIdentity(level roledata.PermissionLevel, denied ...string) xauthz.Identity {
	grants := xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeSettings: level},
	}
	if len(denied) > constants.DefaultInitValue {
		grants.Denied = map[string][]string{roledata.ScopeSettings: denied}
	}
	return xauthz.Identity{UserID: callerID, Grants: grants}
}

func patchRequest(identity xauthz.Identity) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/config", nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

// The route's own check is Contributor, so without this guard a Contributor
// could write every part of the config, including the AI keys the role model
// reserves for Owner.
func TestGuardConfigPatchLevelPerField(t *testing.T) {
	tests := []struct {
		name  string
		level roledata.PermissionLevel
		field string
		want  bool
	}{
		{"contributor edits discovery config", roledata.PermissionLevelContributor, telarkconfig.FieldExcludedNamespaces, true},
		{"contributor edits snapshot storage", roledata.PermissionLevelContributor, telarkconfig.FieldSnapshots, true},
		{"contributor cannot touch ai", roledata.PermissionLevelContributor, telarkconfig.FieldAI, false},
		{"owner edits ai", roledata.PermissionLevelOwner, telarkconfig.FieldAI, true},
		{"owner cannot touch oidc", roledata.PermissionLevelOwner, telarkconfig.FieldOIDC, false},
		{"settings admin cannot touch oidc", roledata.PermissionLevelAdmin, telarkconfig.FieldOIDC, false},
		{"readonly edits nothing", roledata.PermissionLevelReadOnly, telarkconfig.FieldExcludedNamespaces, false},
		{"contributor edits fetch interval", roledata.PermissionLevelContributor, telarkconfig.FieldUserSettings, true},
		{"readonly cannot touch fetch interval", roledata.PermissionLevelReadOnly, telarkconfig.FieldUserSettings, false},
		{"admin cannot touch cluster", roledata.PermissionLevelAdmin, telarkconfig.FieldCluster, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			spec := map[string]any{tt.field: "value"}

			got := authz.GuardConfigPatch(w, patchRequest(settingsIdentity(tt.level)), spec)

			if got != tt.want {
				t.Errorf("GuardConfigPatch() = %v, want %v", got, tt.want)
			}
			if !tt.want && w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

// The three settings rules an admin can untick on a role must actually bite.
func TestGuardConfigPatchHonorsDenyRules(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		action string
		level  roledata.PermissionLevel
	}{
		{"discovery config denied", telarkconfig.FieldExcludedNamespaces, roledata.ActionEditDiscoveryConfig, roledata.PermissionLevelOwner},
		{"snapshot storage denied", telarkconfig.FieldSnapshots, roledata.ActionEditSnapshotStorage, roledata.PermissionLevelOwner},
		{"ai insights denied", telarkconfig.FieldAI, roledata.ActionControlAIInsights, roledata.PermissionLevelOwner},
		{"fetch interval denied", telarkconfig.FieldUserSettings, roledata.ActionEditDiscoveryConfig, roledata.PermissionLevelOwner},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			rule := xauthz.RuleKey(roledata.ScopeSettings, tt.action)
			identity := settingsIdentity(tt.level, rule)

			if authz.GuardConfigPatch(w, patchRequest(identity), map[string]any{tt.field: "value"}) {
				t.Errorf("deny rule %q did not block the field", rule)
			}
		})
	}
}

// Denying one setting must not withhold the others.
func TestGuardConfigPatchDenyIsPerField(t *testing.T) {
	w := httptest.NewRecorder()
	rule := xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditSnapshotStorage)
	identity := settingsIdentity(roledata.PermissionLevelOwner, rule)

	spec := map[string]any{telarkconfig.FieldExcludedNamespaces: []any{"kube-system"}}
	if !authz.GuardConfigPatch(w, patchRequest(identity), spec) {
		t.Error("a deny on snapshot storage blocked discovery config")
	}
}

// A body touching several parts must satisfy the strictest of them.
func TestGuardConfigPatchChecksEveryFieldPresent(t *testing.T) {
	w := httptest.NewRecorder()
	spec := map[string]any{
		telarkconfig.FieldExcludedNamespaces: []any{"kube-system"},
		telarkconfig.FieldAI:                 map[string]any{"enabled": true},
	}

	identity := settingsIdentity(roledata.PermissionLevelContributor)
	if authz.GuardConfigPatch(w, patchRequest(identity), spec) {
		t.Error("an Owner-only field slipped through beside a Contributor one")
	}
}

// The cluster version is reported by discovery; a session, even one holding
// Admin on ALL, never writes it.
func TestGuardConfigPatchClusterIsInternalOnly(t *testing.T) {
	spec := map[string]any{telarkconfig.FieldCluster: map[string]any{"version": "1.31"}}
	for name, identity := range map[string]xauthz.Identity{
		"no scopes":    {UserID: callerID},
		"admin on all": levels(roledata.ScopeAll, roledata.PermissionLevelAdmin),
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			if authz.GuardConfigPatch(w, patchRequest(identity), spec) {
				t.Error("a session wrote the cluster version")
			}
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestGuardConfigPatchDeniesWithoutIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/config", nil)

	if authz.GuardConfigPatch(w, r, map[string]any{telarkconfig.FieldAI: "x"}) {
		t.Error("allowed with no identity in context")
	}
}

func TestGuardConfigPatchAllowsInternalCaller(t *testing.T) {
	w := httptest.NewRecorder()
	identity := xauthz.Identity{Internal: true}

	spec := map[string]any{telarkconfig.FieldCluster: map[string]any{"version": "1.31"}}
	if !authz.GuardConfigPatch(w, patchRequest(identity), spec) {
		t.Error("internal caller was blocked")
	}
}

// Whoever controls the identity settings decides who can sign in, so auth is their
// only writer: every session is refused, Admin on ALL included.
func TestGuardConfigPatchIdentitySettingsAreInternalOnly(t *testing.T) {
	tests := []struct {
		name     string
		identity xauthz.Identity
		want     bool
	}{
		{"admin on ALL", levels(roledata.ScopeAll, roledata.PermissionLevelAdmin), false},
		{"admin on settings only", settingsIdentity(roledata.PermissionLevelAdmin), false},
		{"internal", internalIdentity, true},
	}
	// The JWK is stored in a Secret, not the CR, but it is the same trust decision.
	bodies := map[string]map[string]any{
		"oidc flags":        {telarkconfig.FieldOIDC: map[string]any{keyEnabled: true}},
		"oidc jwk only":     {telarkconfig.FieldOIDC: map[string]any{telarkconfig.OIDCSecretKey: `{"keys":[]}`}},
		"self-registration": {telarkconfig.FieldSelfRegistration: map[string]any{keyEnabled: true}},
	}
	for _, tt := range tests {
		for bodyName, spec := range bodies {
			t.Run(tt.name+"/"+bodyName, func(t *testing.T) {
				w := httptest.NewRecorder()
				got := authz.GuardConfigPatch(w, patchRequest(tt.identity), spec)
				if got != tt.want {
					t.Fatalf("GuardConfigPatch = %v, want %v (%d)", got, tt.want, w.Code)
				}
				if !tt.want && w.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
				}
			})
		}
	}
}

// Seen live: PATCH config {} answered 200 with the whole config, OIDC client id
// and trust set included, to callers GET config refuses.
func TestGuardConfigPatchWithoutGovernedFieldNeedsRead(t *testing.T) {
	tests := []struct {
		name     string
		identity xauthz.Identity
		want     bool
	}{
		{"no settings scope", levels(roledata.ScopeApplications, roledata.PermissionLevelOwner), false},
		{"no scopes", xauthz.Identity{UserID: callerID}, false},
		{"settings read-only", settingsIdentity(roledata.PermissionLevelReadOnly), true},
		{"admin on ALL", allAdmin(), true},
		{"internal", internalIdentity, true},
	}
	for _, tt := range tests {
		for bodyName, spec := range map[string]map[string]any{"empty": {}, "unknown key": {"foo": true}} {
			t.Run(tt.name+"/"+bodyName, func(t *testing.T) {
				w := httptest.NewRecorder()
				if got := authz.GuardConfigPatch(w, patchRequest(tt.identity), spec); got != tt.want {
					t.Fatalf("GuardConfigPatch = %v, want %v (%d)", got, tt.want, w.Code)
				}
				if !tt.want && w.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
				}
			})
		}
	}
}
