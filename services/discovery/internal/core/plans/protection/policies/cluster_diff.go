package policies

import (
	"context"
	"fmt"

	"github.com/telark/data/plans"
	datapolicies "github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/shared"
)

type DiffLogger interface {
	Info(msg string)
	Error(msg string)
}

type DeployErrorMessages struct {
	GenericDeployFailure string
	InternalErrorFormat  string
}

// Order is deploy → mode-patch → delete so coverage never dips during the window.
func ApplyClusterDiff(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	plan *plans.ProtectionPlan,
	deployCombos, removeCombos []PolicyTargetCombo,
	resolved map[string]datapolicies.ResolvedApp,
	newMode string,
	msgs DeployErrorMessages,
) (deployed, kept []string, err error) {
	if plan.Phase != plans.PhaseActive {
		return nil, plan.RenderedPolicies, nil
	}

	rendered, err := renderCombos(plan, deployCombos, resolved, logger)
	if err != nil {
		shared.LogDeployFailure(logger, plan, "update-render", err)
		return nil, nil, err
	}

	deployedNames, err := applier.Deploy(ctx, rendered)
	if err != nil {
		shared.LogDeployFailure(logger, plan, "update-deploy", err)
		_ = applier.DeletePoliciesByNamespacedName(ctx, renderedPolicyRefs(rendered))
		return nil, nil, fmt.Errorf(msgs.InternalErrorFormat, msgs.GenericDeployFailure)
	}

	if newMode != plan.Mode {
		if err := applier.PatchPoliciesMode(ctx, plan.ID, newMode); err != nil {
			shared.LogDeployFailure(logger, plan, "update-mode-patch", err)
			_ = applier.DeletePoliciesByNamespacedName(ctx, renderedPolicyRefs(rendered))
			return nil, nil, fmt.Errorf(msgs.InternalErrorFormat, msgs.GenericDeployFailure)
		}
	}

	removeNames, err := computeRemovalNames(plan, removeCombos, resolved)
	if err != nil {
		_ = applier.DeletePoliciesByNamespacedName(ctx, renderedPolicyRefs(rendered))
		return nil, nil, err
	}
	if err := applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, removeNames); err != nil {
		logger.Error(fmt.Sprintf("protection-plan update remove failed plan=%s err=%v", plan.ID, err))
	}

	kept = subtract(plan.RenderedPolicies, removeNames)
	return deployedNames, kept, nil
}

func RollbackPatchFailure(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	plan *plans.ProtectionPlan,
	deployedNow []string,
) {
	if len(deployedNow) == constants.DefaultInitValue {
		return
	}
	logger.Error(fmt.Sprintf(
		"protection-plan update CRD patch failed; rolling back cluster changes plan=%s deployed=%d",
		plan.ID, len(deployedNow),
	))
	_ = applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, deployedNow)
}

func UnionRenderedNames(kept, deployed []string) []string {
	seen := make(map[string]struct{}, len(kept)+len(deployed))
	out := make([]string, constants.DefaultInitValue, len(kept)+len(deployed))
	for _, n := range kept {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	for _, n := range deployed {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func subtract(all, drop []string) []string {
	dropSet := make(map[string]struct{}, len(drop))
	for _, item := range drop {
		dropSet[item] = struct{}{}
	}
	out := make([]string, constants.DefaultInitValue, len(all))
	for _, item := range all {
		if _, ok := dropSet[item]; ok {
			continue
		}
		out = append(out, item)
	}
	return out
}
