package authz

import (
	"net/http"
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
)

const planKeyPhase = "phase"

func planContributor() xauthz.Identity {
	return xauthz.Identity{
		UserID: callerID,
		Grants: xauthz.Grants{
			Levels: map[string]roledata.PermissionLevel{roledata.ScopeProtectionPlans: roledata.PermissionLevelContributor},
		},
	}
}

func planRequest(identity xauthz.Identity) *http.Request {
	r := httptest.NewRequest(http.MethodPatch, "/v1/plans/protection/p-1", nil)
	return r.WithContext(xauthz.WithIdentity(r.Context(), identity))
}

func lifecycleBodies() map[string]map[string]any {
	return map[string]map[string]any{
		planKeyPhase:   {planKeyPhase: "active"},
		"approval":     {"approval": map[string]any{"state": "approved"}},
		"approvalMode": {"approvalMode": "automatic"},
		"reason":       {"reason": "forged"},
		"healthDetail": {"healthDetail": []any{}},
		// Renames go through discovery's update, which keeps names unique.
		"name": {"name": "taken"},
	}
}

func materialBodies() map[string]map[string]any {
	return map[string]map[string]any{
		"mode":      {"mode": "enforce"},
		"timeRange": {"timeRange": map[string]any{"startAt": "2026-01-01T00:00:00Z"}},
		"policies":  {"policies": []any{}},
	}
}

func assertDenied(t *testing.T, w *httptest.ResponseRecorder, allowed bool, key string) {
	t.Helper()
	if allowed {
		t.Fatalf("session wrote %s", key)
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d for %s, want 403", w.Code, key)
	}
}

func TestGuardPlanLifecycleSessionDeniedOnLifecycleKeys(t *testing.T) {
	for key, body := range lifecycleBodies() {
		w := httptest.NewRecorder()
		assertDenied(t, w, authz.GuardPlanLifecycle(w, planRequest(planContributor()), body), key)
	}
}

func TestGuardPlanLifecycleSessionDeniedOnMaterialKeys(t *testing.T) {
	for key, body := range materialBodies() {
		w := httptest.NewRecorder()
		assertDenied(t, w, authz.GuardPlanLifecycle(w, planRequest(planContributor()), body), key)
	}
}

func TestGuardPlanLifecyclePassesPlainFields(t *testing.T) {
	body := map[string]any{
		"description":     "desc",
		"severity":        "high",
		"priority":        "p1",
		"participantRefs": []any{callerID},
		"environmentRef":  "cat-00002-0001-0001",
		"tagRefs":         []any{},
	}
	w := httptest.NewRecorder()

	if !authz.GuardPlanLifecycle(w, planRequest(planContributor()), body) {
		t.Fatalf("plain fields denied: %d %s", w.Code, w.Body.String())
	}
}

func TestGuardPlanLifecycleInternalPasses(t *testing.T) {
	internal := xauthz.Identity{Internal: true}
	for _, bodies := range []map[string]map[string]any{lifecycleBodies(), materialBodies()} {
		for key, body := range bodies {
			w := httptest.NewRecorder()
			if !authz.GuardPlanLifecycle(w, planRequest(internal), body) {
				t.Fatalf("internal denied on %s", key)
			}
		}
	}
}

func TestGuardPlanLifecycleSessionDeniedOnScopeExclusions(t *testing.T) {
	body := map[string]any{"scope": map[string]any{"exclusions": map[string]any{"kinds": []any{"Deployment"}}}}
	w := httptest.NewRecorder()
	assertDenied(t, w, authz.GuardPlanLifecycle(w, planRequest(planContributor()), body), "scope.exclusions")

	w = httptest.NewRecorder()
	if !authz.GuardPlanLifecycle(w, planRequest(xauthz.Identity{Internal: true}), body) {
		t.Fatalf("internal denied on scope.exclusions: %d %s", w.Code, w.Body.String())
	}
}

func TestGuardPlanLifecycleMissingIdentity(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, "/v1/plans/protection/p-1", nil)

	assertDenied(t, w, authz.GuardPlanLifecycle(w, r, map[string]any{planKeyPhase: "active"}), planKeyPhase)
}
