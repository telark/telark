package health

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
)

// Reporting drift is not enough: a plan whose policies were deleted or flipped to Audit
// protects nothing, so every active check repairs the cluster before storing the health.
func ComputeAndRepair(ctx context.Context, deps Deps, plan *plans.ProtectionPlan) (Result, error) {
	result, err := Compute(ctx, deps.Dyn, plan)
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
	// Cancel/Terminate delete the policies before the phase patch lands, and this
	// path is not leader-gated: without a fresh phase read a concurrent check would
	// redeploy them onto a plan that is about to become terminal, orphaning them.
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
	deps.Logger.Info(fmt.Sprintf(logRepairedFmt, plan.ID, outcome.redeployed, outcome.repatched, outcome.removed))

	repaired, err := Compute(ctx, deps.Dyn, plan)
	if err != nil {
		return result, nil
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
	return len(result.Missing)+len(result.Mismatched)+len(result.Unexpected) > constants.DefaultInitValue
}

func repair(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	result Result,
) (repairOutcome, error) {
	outcome := repairOutcome{}
	if len(result.Missing) > constants.DefaultInitValue {
		if err := redeploy(ctx, deps, plan, result.Missing); err != nil {
			return outcome, err
		}
		outcome.redeployed = result.Missing
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

func redeploy(ctx context.Context, deps Deps, plan *plans.ProtectionPlan, names []string) error {
	resolved, err := resolvePlanScope(ctx, deps, plan)
	if err != nil {
		return err
	}
	rendered, err := policies.Render(plan, resolved, deps.Logger)
	if err != nil {
		return err
	}
	wanted := indexNames(names)
	selected := make([]kyvernov1.Policy, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		if _, ok := wanted[rendered[i].Name]; ok {
			selected = append(selected, rendered[i])
		}
	}
	_, err = deps.Applier.Deploy(ctx, selected)
	return err
}

func resolvePlanScope(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
) (map[string]policies.ResolvedApp, error) {
	if plan.Scope.Type != plans.ScopeTypeApplications {
		return nil, nil
	}
	resolved, missing, err := deps.ResolveApps(ctx, plan.Scope.ApplicationIDs)
	if err != nil {
		return nil, err
	}
	if len(missing) > constants.DefaultInitValue {
		return nil, fmt.Errorf(errMissingAppsFmt, missing)
	}
	return resolved, nil
}
