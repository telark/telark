package controllers

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
)

const (
	maxConcurrentPatches = 2
	userType             = "user"
	userID               = "u1"
	usersField           = "assignedUsersIds"
	stepPurge            = "purge"
	stepList             = "list"
	stepPurgeLast        = "purge-last"
	stepFinalizer        = "finalizer"
)

// ReconcileOne clears a deleted resource's back-references then removes its
// finalizer. The fake target models a group that references the user until it is
// patched, so the happy path (patch → re-confirm cleared → drop finalizer) runs
// end-to-end without a cluster.
func TestReconcileOne(t *testing.T) {
	patched := false
	target := cleanupctrl.Target{
		ResourceType: userType,
		Finalizer:    "auth/user-cleanup",
		BackRefs: []cleanupctrl.BackRef{{
			ResourceType: "group",
			ArrayField:   usersField,
			List: func(_ context.Context) ([]*resourcesshared.CleanupView, error) {
				members := []string{userID}
				if patched {
					members = nil
				}
				return []*resourcesshared.CleanupView{
					{Name: "g1", Refs: map[string][]string{usersField: members}},
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

	cfg := config.CleanupConfig{ReconcilePassDeadline: time.Second, MaxConcurrentPatches: maxConcurrentPatches}
	r := cleanupctrl.NewReconciler(cfg, map[string]cleanupctrl.Target{userType: target}, constants.GetLogger(constants.LoggerPrefixCleanup))

	out, err := r.ReconcileOne(context.Background(), userType, userID, constants.DefaultIncrementValue)
	if err != nil || out.Requeue {
		t.Fatalf("ReconcileOne = (%+v, %v), want success", out, err)
	}
	if out.PatchCount != constants.DefaultIncrementValue {
		t.Fatalf("PatchCount = %d, want 1", out.PatchCount)
	}
	if !patched {
		t.Fatal("back-reference was never patched")
	}

	if _, err := r.ReconcileOne(context.Background(), "unknown", "x", constants.DefaultIncrementValue); err == nil {
		t.Fatal("unknown resource type should error")
	}
}

// A target's owned records (a user's sessions) are purged before any
// back-reference is touched, and a purge failure requeues without dropping the finalizer.
func TestReconcileOnePurgesOwnedRecordsFirst(t *testing.T) {
	var order []string
	finalizerRemoved := false
	purgeErr := errors.New("exporter down")
	target := cleanupctrl.Target{
		ResourceType: userType,
		Finalizer:    "auth/user-cleanup",
		Purge: func(_ string) error {
			order = append(order, stepPurge)
			return purgeErr
		},
		BackRefs: []cleanupctrl.BackRef{{
			ResourceType: "group",
			ArrayField:   usersField,
			List: func(_ context.Context) ([]*resourcesshared.CleanupView, error) {
				order = append(order, stepList)
				return nil, nil
			},
		}},
		RemoveFinalizer: func(_ context.Context, _, _ string) *response.GenericResponse {
			finalizerRemoved = true
			return &response.GenericResponse{Status: http.StatusOK}
		},
	}
	cfg := config.CleanupConfig{ReconcilePassDeadline: time.Second, MaxConcurrentPatches: maxConcurrentPatches}
	r := cleanupctrl.NewReconciler(cfg, map[string]cleanupctrl.Target{userType: target}, constants.GetLogger(constants.LoggerPrefixCleanup))

	out, err := r.ReconcileOne(context.Background(), userType, userID, constants.DefaultIncrementValue)
	if !errors.Is(err, purgeErr) || !out.Requeue {
		t.Fatalf("ReconcileOne = (%+v, %v), want requeue on purge failure", out, err)
	}
	if finalizerRemoved || !slices.Equal(order, []string{stepPurge}) {
		t.Fatalf("order = %v, finalizerRemoved = %v; want purge alone, finalizer kept", order, finalizerRemoved)
	}

	purgeErr = nil
	out, err = r.ReconcileOne(context.Background(), userType, userID, constants.DefaultIncrementValue)
	if err != nil || out.Requeue || !finalizerRemoved {
		t.Fatalf("ReconcileOne = (%+v, %v), want success after purge", out, err)
	}
	if !slices.Equal(order, []string{stepPurge, stepPurge, stepList, stepList}) {
		t.Fatalf("order = %v, want purge before the back-reference clean and confirm passes", order)
	}
}

// Records that clearing a back-reference can write are purged only once no reference is
// left, before the finalizer goes; a failure there requeues with the finalizer kept.
func TestReconcileOnePurgesLastAfterRefsClear(t *testing.T) {
	var order []string
	lastErr := errors.New("exporter down")
	target := cleanupctrl.Target{
		ResourceType: userType,
		BackRefs: []cleanupctrl.BackRef{{
			ArrayField: usersField,
			List: func(_ context.Context) ([]*resourcesshared.CleanupView, error) {
				order = append(order, stepList)
				return nil, nil
			},
		}},
		PurgeLast: func(_ string) error {
			order = append(order, stepPurgeLast)
			return lastErr
		},
		RemoveFinalizer: func(_ context.Context, _, _ string) *response.GenericResponse {
			order = append(order, stepFinalizer)
			return &response.GenericResponse{Status: http.StatusOK}
		},
	}
	cfg := config.CleanupConfig{ReconcilePassDeadline: time.Second, MaxConcurrentPatches: maxConcurrentPatches}
	r := cleanupctrl.NewReconciler(cfg, map[string]cleanupctrl.Target{userType: target}, constants.GetLogger(constants.LoggerPrefixCleanup))

	out, err := r.ReconcileOne(context.Background(), userType, userID, constants.DefaultIncrementValue)
	if !errors.Is(err, lastErr) || !out.Requeue || !slices.Equal(order, []string{stepList, stepList, stepPurgeLast}) {
		t.Fatalf("ReconcileOne = (%+v, %v), order %v; want a requeue after the confirm pass, finalizer kept", out, err, order)
	}

	lastErr, order = nil, nil
	out, err = r.ReconcileOne(context.Background(), userType, userID, constants.DefaultIncrementValue)
	if err != nil || out.Requeue || !slices.Equal(order, []string{stepList, stepList, stepPurgeLast, stepFinalizer}) {
		t.Fatalf("ReconcileOne = (%+v, %v), order %v; want the last purge just before the finalizer", out, err, order)
	}
}
