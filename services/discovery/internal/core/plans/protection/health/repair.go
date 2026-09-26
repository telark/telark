package health

import (
	"context"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
)

// Reporting drift is not enough: a plan whose policies were deleted or flipped to Audit
// protects nothing, so every active check repairs the cluster before storing the health.
func ComputeAndRepair(ctx context.Context, deps Deps, plan *plans.ProtectionPlan) (Result, error) {
	result, err := Compute(ctx, deps, plan)
	if err != nil {
		return Result{}, err
	}
	return repairIfDrifted(ctx, deps, plan, result)
}

func repairIfDrifted(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	result Result,
) (Result, error) {
	if plan.Phase != plans.PhaseActive || !needsRepair(result) {
		return result, nil
	}
	// The controller's plan list can predate a Cancel or Terminate: without a fresh phase read
	// the repair would redeploy policies onto a plan that just became terminal, orphaning them.
	stillActive, err := activeAtSource(deps, plan.ID)
	if err != nil {
		deps.Logger.Error(formatErr(stagePhaseRecheck, plan.ID, err))
		return result, nil
	}
	if !stillActive {
		return result, nil
	}

	outcome, repairErr := repair(ctx, deps, plan, result)
	if repairErr != nil {
		deps.Logger.Error(formatErr(stageRepair, plan.ID, repairErr))
		return result, nil
	}
	deps.Logger.Info(fmt.Sprintf(logRepairedFmt, plan.ID, outcome.redeployed, outcome.repatched, outcome.removed, outcome.added))
	if len(outcome.added) > constants.DefaultInitValue {
		plan.RenderedPolicies = slices.Concat(plan.RenderedPolicies, outcome.added)
	}

	repaired, err := Compute(ctx, deps, plan)
	if err != nil {
		return result, nil
	}
	if len(outcome.added) > constants.DefaultInitValue {
		repaired.Rendered = plan.RenderedPolicies
	}
	return repaired, nil
}

func activeAtSource(deps Deps, planID string) (bool, error) {
	current, err := deps.Exporter.Get(planID)
	if err != nil {
		return false, err
	}
	return current.Phase == plans.PhaseActive, nil
}

func needsRepair(result Result) bool {
	return len(result.Missing)+len(result.Mismatched)+len(result.Unexpected)+len(result.Stale)+len(result.Added) >
		constants.DefaultInitValue
}

func repair(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	result Result,
) (repairOutcome, error) {
	outcome := repairOutcome{}
	redeploy := slices.Concat(result.Missing, result.Stale)
	if len(redeploy)+len(result.Added) > constants.DefaultInitValue {
		if err := deployCurrent(ctx, deps, result, slices.Concat(redeploy, result.Added)); err != nil {
			return outcome, err
		}
		outcome.redeployed = redeploy
		outcome.added = result.Added
	}
	if len(result.Mismatched) > constants.DefaultInitValue {
		if err := deps.Applier.PatchPoliciesMode(ctx, plan.ID, plan.Mode); err != nil {
			return outcome, err
		}
		outcome.repatched = result.Mismatched
	}
	if len(result.Unexpected) > constants.DefaultInitValue {
		if err := deps.Applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, result.Unexpected); err != nil {
			return outcome, err
		}
		outcome.removed = result.Unexpected
	}
	return outcome, nil
}

func deployCurrent(ctx context.Context, deps Deps, result Result, names []string) error {
	if result.current == nil {
		return fmt.Errorf(fmtRenderUnavailable, result.renderErr)
	}
	selected := make([]kyvernov1.Policy, constants.DefaultInitValue, len(names))
	for _, name := range names {
		if pol, ok := result.current[name]; ok {
			selected = append(selected, pol)
		}
	}
	_, err := deps.Applier.Deploy(ctx, selected)
	return err
}
