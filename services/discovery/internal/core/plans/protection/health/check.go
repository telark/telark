package health

import (
	"context"
	"time"

	"github.com/telark/data/plans"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/dynamic"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type Deps struct {
	Exporter *clients.ProtectionPlanClient
	Dyn      dynamic.Interface
	Logger   Logger
	Clock    func() time.Time
	System   string
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

	result, err := Compute(checkCtx, deps.Dyn, plan)
	if err != nil {
		return plan, Result{}, err
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

// Errors per plan are logged and the loop continues — one bad plan must not block the rest.
func ReconcileForActive(ctx context.Context, deps Deps, planList []plans.ProtectionPlan) {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.HealthReconcileConcurrency)
	for i := range planList {
		plan := &planList[i]
		if plan.Phase != plans.PhaseActive {
			continue
		}
		g.Go(func() error {
			reconcileOne(gctx, deps, plan)
			return nil
		})
	}
	_ = g.Wait()
}

func reconcileOne(ctx context.Context, deps Deps, plan *plans.ProtectionPlan) {
	checkCtx, cancel := context.WithTimeout(ctx, time.Duration(CheckTimeoutSeconds)*time.Second)
	defer cancel()

	result, err := Compute(checkCtx, deps.Dyn, plan)
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
