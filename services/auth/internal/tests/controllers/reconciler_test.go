package controllers

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	resourcesshared "github.com/telark/data/resources/shared"
	"github.com/telark/rest/response"
)

// ReconcileOne clears a deleted resource's back-references then removes its
// finalizer. The fake target models a group that references the user until it is
// patched, so the happy path (patch → re-confirm cleared → drop finalizer) runs
// end-to-end without a cluster.
func TestReconcileOne(t *testing.T) {
	patched := false
	target := cleanupctrl.Target{
		ResourceType: "user",
		Finalizer:    "auth/user-cleanup",
		BackRefs: []cleanupctrl.BackRef{{
			ResourceType: "group",
			ArrayField:   "assignedUsersIds",
			List: func(_ context.Context) ([]*resourcesshared.CleanupView, error) {
				members := []string{"u1"}
				if patched {
					members = nil
				}
				return []*resourcesshared.CleanupView{
					{Name: "g1", Refs: map[string][]string{"assignedUsersIds": members}},
				}, nil
			},
			Patch: func(_ context.Context, _ string, _ map[string]any) *response.GenericResponse {
				patched = true
				return &response.GenericResponse{Status: http.StatusOK}
			},
		}},
		RemoveFinalizer: func(_ context.Context, _, _ string) *response.GenericResponse {
			return &response.GenericResponse{Status: http.StatusOK}
		},
	}

	cfg := config.CleanupConfig{ReconcilePassDeadline: time.Second, MaxConcurrentPatches: 2}
	r := cleanupctrl.NewReconciler(cfg, map[string]cleanupctrl.Target{"user": target}, constants.GetLogger(constants.LoggerPrefixCleanup))

	out, err := r.ReconcileOne(context.Background(), "user", "u1", 1)
	if err != nil || out.Requeue {
		t.Fatalf("ReconcileOne = (%+v, %v), want success", out, err)
	}
	if out.PatchCount != 1 {
		t.Fatalf("PatchCount = %d, want 1", out.PatchCount)
	}
	if !patched {
		t.Fatal("back-reference was never patched")
	}

	if _, err := r.ReconcileOne(context.Background(), "unknown", "x", 1); err == nil {
		t.Fatal("unknown resource type should error")
	}
}
