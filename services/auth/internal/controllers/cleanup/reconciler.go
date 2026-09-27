package cleanup

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/data/logger"
	resourcesshared "github.com/telark/data/resources/shared"
)

func NewReconciler(cfg config.CleanupConfig, targets map[string]Target, lg *logger.CustomLogger) *Reconciler {
	return &Reconciler{cfg: cfg, targets: targets, lg: lg}
}

type Outcome struct {
	Requeue    bool
	PatchCount int
	DurationMS int64
}

func (r *Reconciler) ReconcileOne(ctx context.Context, resourceType, id string, attempts int) (Outcome, error) {
	target, ok := r.targets[resourceType]
	if !ok {
		return Outcome{}, fmt.Errorf(string(constants.ErrCleanupUnknownResourceType), resourceType)
	}
	r.lg.Debug(fmt.Sprintf(string(constants.LogCleanupReconcileStart), resourceType, id, attempts))
	start := time.Now()

	passCtx, cancel := context.WithTimeout(ctx, r.cfg.ReconcilePassDeadline)
	defer cancel()

	if target.Purge != nil {
		if err := target.Purge(id); err != nil {
			r.lg.Error(fmt.Sprintf(string(constants.LogCleanupReconcileFail), resourceType, id, attempts, err))
			return Outcome{Requeue: true, DurationMS: elapsedMS(start)}, err
		}
	}

	patchCount := constants.DefaultInitValue
	for _, ref := range target.BackRefs {
		if err := passCtx.Err(); err != nil {
			return Outcome{Requeue: true, DurationMS: elapsedMS(start)}, err
		}
		n, err := r.cleanBackRef(passCtx, ref, id)
		patchCount += n
		if err != nil {
			r.lg.Error(fmt.Sprintf(string(constants.LogCleanupReconcileFail), resourceType, id, attempts, err))
			return Outcome{Requeue: true, PatchCount: patchCount, DurationMS: elapsedMS(start)}, err
		}
	}

	if err := passCtx.Err(); err != nil {
		return Outcome{Requeue: true, PatchCount: patchCount, DurationMS: elapsedMS(start)}, err
	}

	if err := confirmAllRefsCleared(passCtx, target, id); err != nil {
		return Outcome{Requeue: true, PatchCount: patchCount, DurationMS: elapsedMS(start)}, err
	}

	if err := r.removeFinalizer(passCtx, target, id); err != nil {
		return Outcome{Requeue: true, PatchCount: patchCount, DurationMS: elapsedMS(start)}, err
	}

	r.lg.Debug(fmt.Sprintf(string(constants.LogCleanupReconcileDone), resourceType, id, elapsedMS(start), patchCount))
	return Outcome{Requeue: false, PatchCount: patchCount, DurationMS: elapsedMS(start)}, nil
}

func (r *Reconciler) cleanBackRef(ctx context.Context, ref BackRef, targetID string) (int, error) {
	views, err := ref.List(ctx)
	if err != nil {
		return constants.DefaultInitValue, fmt.Errorf(
			string(constants.ErrCleanupListBackRefsFailed), ref.ResourceType, err)
	}
	affected := filterByMembership(views, ref.ArrayField, targetID)
	if len(affected) == constants.DefaultInitValue {
		return constants.DefaultInitValue, nil
	}
	return r.patchAffected(ctx, ref, targetID, affected)
}

func (r *Reconciler) patchAffected(
	ctx context.Context,
	ref BackRef,
	targetID string,
	affected []*resourcesshared.CleanupView,
) (int, error) {
	sem := make(chan struct{}, r.cfg.MaxConcurrentPatches)
	var wg sync.WaitGroup
	var firstErr atomic.Pointer[error]
	var successCount atomic.Int64

	for _, view := range affected {
		if err := ctx.Err(); err != nil {
			return int(successCount.Load()), err
		}
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			if err := r.patchOne(ctx, ref, view, targetID); err != nil {
				e := err
				firstErr.CompareAndSwap(nil, &e)
				return
			}
			successCount.Add(constants.DefaultIncrementValue)
		})
	}
	wg.Wait()
	if errPtr := firstErr.Load(); errPtr != nil {
		return int(successCount.Load()), *errPtr
	}
	return int(successCount.Load()), nil
}

func (*Reconciler) patchOne(
	ctx context.Context,
	ref BackRef,
	view *resourcesshared.CleanupView,
	targetID string,
) error {
	current := view.Refs[ref.ArrayField]
	if !slices.Contains(current, targetID) {
		return nil
	}
	next := slices.DeleteFunc(slices.Clone(current), func(s string) bool { return s == targetID })
	body := map[string]any{ref.ArrayField: next}
	resp := ref.Patch(ctx, view.Name, body)
	if resp == nil || resp.Status != http.StatusOK {
		status := constants.DefaultInitValue
		if resp != nil {
			status = resp.Status
		}
		return fmt.Errorf(string(constants.ErrCleanupPatchBackRefFailed), ref.ResourceType, view.Name, status)
	}
	return nil
}

func confirmAllRefsCleared(ctx context.Context, target Target, id string) error {
	for _, ref := range target.BackRefs {
		if err := ctx.Err(); err != nil {
			return err
		}
		views, err := ref.List(ctx)
		if err != nil {
			return err
		}
		if len(filterByMembership(views, ref.ArrayField, id)) > constants.DefaultInitValue {
			return fmt.Errorf(string(constants.ErrCleanupRefsStillPresent), ref.ResourceType)
		}
	}
	return nil
}

func (r *Reconciler) removeFinalizer(ctx context.Context, target Target, id string) error {
	resp := target.RemoveFinalizer(ctx, id, target.Finalizer)
	if resp == nil {
		return fmt.Errorf(
			string(constants.ErrCleanupRemoveFinalizerFail),
			target.ResourceType, id, constants.DefaultInitValue,
		)
	}
	if resp.Status != http.StatusOK && resp.Status != http.StatusNotFound {
		return fmt.Errorf(string(constants.ErrCleanupRemoveFinalizerFail), target.ResourceType, id, resp.Status)
	}
	r.lg.Info(fmt.Sprintf(string(constants.LogCleanupFinalizerRemoved), target.ResourceType, id))
	return nil
}

func filterByMembership(views []*resourcesshared.CleanupView, arrayField, id string) []*resourcesshared.CleanupView {
	out := make([]*resourcesshared.CleanupView, constants.DefaultInitValue, len(views))
	for _, v := range views {
		if v == nil {
			continue
		}
		if slices.Contains(v.Refs[arrayField], id) {
			out = append(out, v)
		}
	}
	return out
}

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}
