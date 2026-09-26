package health

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/telark/data/plans"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/dynamic"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type Deps struct {
	Exporter    PlanStore
	Dyn         dynamic.Interface
	Applier     *protpolicies.Applier
	ResolveApps applications.Resolver
	Logger      Logger
	Clock       func() time.Time
	System      string
}

// The PATCH is best-effort: a failed persist is logged, not returned, because the health read
// itself still succeeded.
func Check(ctx context.Context, deps Deps, planID string) (*plans.ProtectionPlan, Result, error) {
	plan, err := deps.Exporter.Get(planID)
	if err != nil {
		return nil, Result{}, err
	}

	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(CheckTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := ComputeAndRepair(checkCtx, deps, plan)
	if err != nil {
		return plan, Result{}, err
	}
	// Every non-active transition already stores health=unknown; patching it again only churned
	// lastUpdatedAt/By=system on every status poll.
	if plan.Phase != plans.PhaseActive {
		plan.Health = plans.HealthUnknown
		return plan, result, nil
	}

	now := deps.Clock().Format(globalshared.DefaultTimeFormat)
	if patchErr := deps.Exporter.PatchOrError(deps.System, planID, ToPatch(result, now)); patchErr != nil {
		deps.Logger.Error(formatErr("patch", planID, patchErr))
	}

	plan.Health = result.Health
	plan.HealthCheckedAt = &now
	plan.HealthDetail = result.Detail
	return plan, result, nil
}

// A plan that just went active carries no health until the controller's next tick, which shows
// a fully enforcing plan as unknown for up to the tick interval. One delayed check closes that
// window; the delay is there because Kyverno has not marked the Policy Ready yet.
func StampFirst(ctx context.Context, deps Deps, planID string) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(firstCheckDelaySeconds * time.Second):
	}
	// A bulk create would otherwise fire one cluster LIST per plan at the same instant. Anything
	// that cannot get a slot inside its budget is left to the controller tick.
	select {
	case firstCheckSlots <- struct{}{}:
		defer func() { <-firstCheckSlots }()
	case <-ctx.Done():
		return
	}
	// Check re-reads the plan, so a plan canceled in the meantime computes as unknown and the
	// repair path's own phase re-check keeps its policies from being redeployed.
	if _, _, err := Check(ctx, deps, planID); err != nil {
		deps.Logger.Error(formatErr(stageFirstCheck, planID, err))
	}
}

// Errors per plan are logged and the loop continues — one bad plan must not block the rest.
func ReconcileForActive(ctx context.Context, deps Deps, planList []plans.ProtectionPlan) {
	snapCtx, cancelSnap := context.WithTimeout(ctx, time.Duration(CheckTimeoutSeconds)*time.Second)
	snapshot, err := listManagedPolicies(snapCtx, deps.Dyn)
	cancelSnap()
	if err != nil {
		deps.Logger.Error(fmt.Sprintf(logSnapshotFailedFmt, err))
		return
	}

	// One application resolve for the pass, like the one LIST above: the fresh render every
	// active plan is compared against must not cost one application read per plan per tick.
	resolveCtx, cancelResolve := context.WithTimeout(ctx, time.Duration(CheckTimeoutSeconds)*time.Second)
	resolved, resolveErr := resolveApps(resolveCtx, deps, activeApplicationIDs(planList))
	cancelResolve()
	if resolveErr != nil {
		deps.Logger.Error(fmt.Sprintf(logResolveFailedFmt, resolveErr))
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.HealthReconcileConcurrency)
	for i := range planList {
		plan := &planList[i]
		if plan.Phase != plans.PhaseActive {
			continue
		}
		g.Go(func() error {
			reconcileOne(gctx, deps, plan, computeFrom(plan, snapshot[plan.ID], resolved, resolveErr))
			return nil
		})
	}
	_ = g.Wait()
}

func activeApplicationIDs(planList []plans.ProtectionPlan) []string {
	var ids []string
	for i := range planList {
		if planList[i].Phase == plans.PhaseActive {
			ids = append(ids, planApplicationIDs(&planList[i])...)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(ids)))
}

func reconcileOne(ctx context.Context, deps Deps, plan *plans.ProtectionPlan, computed Result) {
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(CheckTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := repairIfDrifted(checkCtx, deps, plan, computed)
	if err != nil {
		deps.Logger.Error(formatErr("compute", plan.ID, err))
		return
	}
	now := deps.Clock().Format(globalshared.DefaultTimeFormat)
	if err := deps.Exporter.PatchOrError(deps.System, plan.ID, ToPatch(result, now)); err != nil {
		deps.Logger.Error(formatErr("patch", plan.ID, err))
	}
}

func formatErr(stage, planID string, err error) string {
	return "protection-plan health " + stage + " failed plan=" + planID + " err=" + err.Error()
}
