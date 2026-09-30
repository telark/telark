package policies

import (
	"context"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/telark/internal/data/plans"
	datapolicies "github.com/telark/telark/internal/data/policies"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/shared"
)

type DiffLogger interface {
	Info(msg string)
	Error(msg string)
}

type DeployErrorMessages struct {
	GenericDeployFailure string
	InternalErrorFormat  string
}

// Order is deploy → mode-patch. The removed names come back as stale, deleted only once the exporter
// patch lands, so coverage never dips during the window and a refused patch leaves them live.
func ApplyClusterDiff(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	plan *plans.ProtectionPlan,
	deployCombos, removeCombos []PolicyTargetCombo,
	resolved map[string]datapolicies.ResolvedApp,
	newMode string,
	msgs DeployErrorMessages,
) (deployed, kept, stale []string, err error) {
	if plan.Phase != plans.PhaseActive {
		return nil, plan.RenderedPolicies, nil, nil
	}

	removeNames, err := computeRemovalNames(plan, removeCombos, resolved)
	if err != nil {
		return nil, nil, nil, err
	}

	rendered, err := renderCombos(plan, deployCombos, resolved, logger)
	if err != nil {
		shared.LogDeployFailure(logger, plan, "update-render", err)
		return nil, nil, nil, err
	}

	deployedNames, err := applier.Deploy(ctx, rendered)
	if err != nil {
		shared.LogDeployFailure(logger, plan, "update-deploy", err)
		_ = applier.DeletePoliciesByNamespacedName(ctx, renderedPolicyRefs(rendered))
		return nil, nil, nil, fmt.Errorf(msgs.InternalErrorFormat, msgs.GenericDeployFailure)
	}

	if newMode != plan.Mode {
		if err := applier.PatchPoliciesMode(ctx, plan.ID, newMode); err != nil {
			shared.LogDeployFailure(logger, plan, "update-mode-patch", err)
			_ = applier.DeletePoliciesByNamespacedName(ctx, renderedPolicyRefs(rendered))
			return nil, nil, nil, fmt.Errorf(msgs.InternalErrorFormat, msgs.GenericDeployFailure)
		}
	}

	// A template re-added with other params renders under its old name: that one stays.
	stale = subtract(removeNames, deployedNames)
	return deployedNames, subtract(plan.RenderedPolicies, stale), stale, nil
}

func RollbackPatchFailure(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	plan *plans.ProtectionPlan,
	deployedNow []string,
	newMode string,
) {
	if len(deployedNow) > constants.DefaultInitValue {
		logger.Error(fmt.Sprintf(
			"protection-plan update CRD patch failed; rolling back cluster changes plan=%s deployed=%d",
			plan.ID, len(deployedNow),
		))
		_ = applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, deployedNow)
	}
	if newMode == plan.Mode {
		return
	}
	if err := applier.PatchPoliciesMode(ctx, plan.ID, plan.Mode); err != nil {
		shared.LogDeployFailure(logger, plan, "update-rollback-mode", err)
	}
}

// A full render reproduces the live names, so nothing is deleted here: Run deletes the stale names
// only after the exporter patch lands, and a failure restores the old render instead.
func ApplyFullRender(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	oldPlan, newPlan *plans.ProtectionPlan,
	resolved map[string]datapolicies.ResolvedApp,
	msgs DeployErrorMessages,
) (deployed, kept, stale []string, err error) {
	if oldPlan.Phase != plans.PhaseActive {
		return nil, oldPlan.RenderedPolicies, nil, nil
	}

	rendered, err := datapolicies.Render(newPlan, resolved, logger)
	if err != nil {
		shared.LogDeployFailure(logger, newPlan, "update-render-full", err)
		return nil, nil, nil, err
	}

	deployedNames, err := applier.Deploy(ctx, rendered)
	if err != nil {
		shared.LogDeployFailure(logger, newPlan, "update-deploy-full", err)
		RollbackFullRender(ctx, applier, logger, oldPlan, policyNames(rendered), resolved)
		return nil, nil, nil, fmt.Errorf(msgs.InternalErrorFormat, msgs.GenericDeployFailure)
	}
	return deployedNames, nil, subtract(oldPlan.RenderedPolicies, deployedNames), nil
}

// Best effort on a path that already failed. SSA already overwrote same-named policies with the
// new content, so the old ones are re-applied, and never a name the CR does not list.
func RollbackFullRender(
	ctx context.Context,
	applier *Applier,
	logger DiffLogger,
	oldPlan *plans.ProtectionPlan,
	attempted []string,
	resolved map[string]datapolicies.ResolvedApp,
) {
	if len(attempted) == constants.DefaultInitValue {
		return
	}
	if err := applier.DeletePoliciesByLabelAndNames(ctx, oldPlan.ID, subtract(attempted, oldPlan.RenderedPolicies)); err != nil {
		shared.LogDeployFailure(logger, oldPlan, "update-rollback-delete", err)
	}
	old, err := datapolicies.Render(oldPlan, resolved, logger)
	if err != nil {
		shared.LogDeployFailure(logger, oldPlan, "update-rollback-render", err)
		return
	}
	old = slices.DeleteFunc(old, func(p kyvernov1.Policy) bool {
		return !slices.Contains(oldPlan.RenderedPolicies, p.Name)
	})
	if _, err := applier.Deploy(ctx, old); err != nil {
		shared.LogDeployFailure(logger, oldPlan, "update-rollback-deploy", err)
	}
}

func UnionRenderedNames(kept, deployed []string) []string {
	all := slices.Concat(kept, deployed)
	seen := make(map[string]struct{}, len(all))
	out := make([]string, constants.DefaultInitValue, len(all))
	for _, n := range all {
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

func policyNames(rendered []kyvernov1.Policy) []string {
	names := make([]string, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		names = append(names, rendered[i].Name)
	}
	return names
}
