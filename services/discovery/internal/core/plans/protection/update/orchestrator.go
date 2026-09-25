package update

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	planseps "github.com/telark/rest/endpoints/plans"
	restmapper "github.com/telark/rest/mappers"
)

type (
	Deps struct {
		Applier        *protpolicies.Applier
		Exporter       *clients.ProtectionPlanClient
		ResolveApps    applications.Resolver
		ListNamespaces validation.NamespaceLister
		Logger         protection.Logger
		Clock          func() time.Time
		// The patch blanks health; this restamps it without waiting for a controller tick.
		StampHealth func(planID string)
		// Called after a material edit of a pending plan re-requests approval.
		NotifyApprovers func(*plans.ProtectionPlan)
	}
)

// Deploy-then-delete keeps coverage at the intersection of old and new during the update
// window, so the plan never drops below its prior posture.
func Run(
	ctx context.Context,
	deps Deps,
	userID, planID string,
	req *planseps.PrepareProtectionPlanRequest,
) (*plans.ProtectionPlan, error) {
	plan, err := deps.Exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if err := validateRequest(ctx, plan, req, deps); err != nil {
		return nil, err
	}

	newPolicies := toPolicies(req.Policies)
	material := MaterialChange(plan, req, newPolicies)
	fullRender := exclusionsChanged(plan.Scope, req.Scope)
	if protection.RequiresApproval(plan) && approvedPhase(plan.Phase) && material {
		return nil, validation.Invalid(string(protection.ErrApprovedPlanMaterialEdit))
	}
	policyDiff := diffPolicies(plan.Policies, newPolicies)
	targetDiff := diffTargets(scopeTargets(plan), newTargets(req.Scope))

	resolved, err := resolveTargets(ctx, deps.ResolveApps, plan.Scope.Type, allTargets(targetDiff))
	if err != nil {
		return nil, err
	}

	deployed, kept, stale, err := applyCluster(ctx, deps, plan, req, newPolicies, policyDiff, targetDiff, resolved, fullRender)
	if err != nil {
		return nil, err
	}

	rendered := protpolicies.UnionRenderedNames(kept, deployed)
	now := deps.Clock().Format(globalshared.DefaultTimeFormat)
	patch, changed := BuildPatch(plan, req, newPolicies, rendered, userID, now)
	if !changed {
		return plan, nil
	}

	reRequest := plan.Phase == plans.PhasePendingApproval && material
	if err := applyPatch(deps, plan, userID, now, patch, reRequest); err != nil {
		rollbackCluster(ctx, deps, plan, deployed, resolved, fullRender)
		return nil, fmt.Errorf(string(ErrPartial), planID)
	}
	if err := deps.Applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, stale); err != nil {
		deps.Logger.Error(fmt.Sprintf(protection.LogStaleDeleteFailed, plan.ID, err))
	}
	return afterPatch(deps, plan, reRequest)
}

// An exclusions change is content under the same names, which the combos diff never re-renders,
// so it deploys the whole new plan instead.
func applyCluster(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
	policyDiff PolicyDiff,
	targetDiff TargetDiff,
	resolved map[string]policies.ResolvedApp,
	fullRender bool,
) (deployed, kept, stale []string, err error) {
	msgs := protpolicies.DeployErrorMessages{
		GenericDeployFailure: deployFailureGeneric,
		InternalErrorFormat:  string(ErrInternal),
	}
	if fullRender {
		return protpolicies.ApplyFullRender(ctx, deps.Applier, deps.Logger, plan, renderTarget(plan, req, newPolicies), resolved, msgs)
	}
	deployCombos := append(
		protpolicies.Combinations(newPolicies, targetDiff.Added),
		protpolicies.Combinations(policyDiff.Added, targetDiff.Unchanged)...,
	)
	removeCombos := append(
		protpolicies.Combinations(plan.Policies, targetDiff.Removed),
		protpolicies.Combinations(policyDiff.Removed, targetDiff.Unchanged)...,
	)
	deployed, kept, err = protpolicies.ApplyClusterDiff(
		ctx, deps.Applier, deps.Logger, plan,
		deployCombos, removeCombos, resolved, req.Mode, msgs,
	)
	return deployed, kept, nil, err
}

// Never RollbackPatchFailure after a full render: it deletes every deployed name, and a full
// render deploys every live policy of the plan.
func rollbackCluster(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	deployed []string,
	resolved map[string]policies.ResolvedApp,
	fullRender bool,
) {
	if fullRender {
		protpolicies.RollbackFullRender(ctx, deps.Applier, deps.Logger, plan, deployed, resolved)
		return
	}
	protpolicies.RollbackPatchFailure(ctx, deps.Applier, deps.Logger, plan, deployed)
}

func renderTarget(
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
) *plans.ProtectionPlan {
	target := *plan
	target.Scope = plans.ProtectionPlanScope{
		Type:           plan.Scope.Type,
		ApplicationIDs: req.Scope.ApplicationIDs,
		Namespaces:     req.Scope.Namespaces,
		Exclusions:     effectiveExclusions(plan.Scope, req.Scope),
	}
	target.Policies = newPolicies
	target.Mode = req.Mode
	return &target
}

func afterPatch(deps Deps, plan *plans.ProtectionPlan, reRequest bool) (*plans.ProtectionPlan, error) {
	if plan.Phase == plans.PhaseActive && deps.StampHealth != nil {
		deps.StampHealth(plan.ID)
	}
	updated, err := deps.Exporter.Get(plan.ID)
	if err != nil {
		return nil, err
	}
	if reRequest && deps.NotifyApprovers != nil {
		deps.NotifyApprovers(updated)
	}
	return updated, nil
}

// A material edit of a pending plan resets the approval request in the same patch,
// so an approver can never act on a version they did not see.
func applyPatch(
	deps Deps,
	plan *plans.ProtectionPlan,
	userID, now string,
	patch planseps.PatchProtectionPlanRequest,
	reRequest bool,
) error {
	if !reRequest && patch.Scope == nil {
		return deps.Exporter.PatchOrError(userID, plan.ID, patch)
	}
	body, err := restmapper.MapToJSONPayload(patch)
	if err != nil {
		return err
	}
	if scope, ok := body[protection.FieldScope].(map[string]any); ok && patch.Scope != nil {
		scope[protection.FieldScopeExclusions] = protection.ExclusionsPatchValue(patch.Scope.Exclusions)
	}
	if reRequest {
		body[protection.FieldApproval] = protection.ApprovalPatchValue(protection.NewApprovalRequest(userID, now, plan.Approval))
	}
	return deps.Exporter.PatchRawOrError(userID, plan.ID, body)
}

func ApprovalModeChanged(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest) bool {
	return req.ApprovalMode != nil &&
		protection.EffectiveApprovalMode(*req.ApprovalMode) != protection.EffectiveApprovalMode(plan.ApprovalMode)
}

func approvedPhase(phase string) bool {
	return phase == plans.PhaseActive || phase == plans.PhaseScheduled
}

func validateRequest(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	deps Deps,
) error {
	if !updatable(plan.Phase) {
		return validation.Invalidf(fmtRawString, ErrInvalidPhase)
	}
	if req.Scope.Type != plan.Scope.Type {
		return validation.Invalidf(fmtRawString, ErrScopeTypeChange)
	}
	if ApprovalModeChanged(plan, req) {
		return validation.Invalid(string(protection.ErrApprovalModeImmutable))
	}
	if err := validation.Fields(req); err != nil {
		return err
	}
	if err := ensureNameAvailable(deps, plan, req.Name); err != nil {
		return err
	}
	if err := validation.NamespaceScope(ctx, req.Scope.Type, req.Scope.Namespaces, deps.ListNamespaces); err != nil {
		return err
	}
	if err := validation.Exclusions(req.Scope); err != nil {
		return err
	}
	if err := validatePolicies(req.Policies, req.Scope.Type); err != nil {
		return validation.Invalidf(string(ErrInvalidPolicies), err)
	}
	if req.TimeMode == plans.TimeModeTimeRange {
		if err := validateTimeRange(req.TimeRange); err != nil {
			return err
		}
	}
	return nil
}

func ensureNameAvailable(deps Deps, plan *plans.ProtectionPlan, name string) error {
	if validation.NormalizeName(name) == validation.NormalizeName(plan.Name) {
		return nil
	}
	existing, err := deps.Exporter.List()
	if err != nil {
		return err
	}
	return validation.UniqueName(existing, name, plan.ID)
}

func resolveTargets(
	ctx context.Context,
	resolveApps applications.Resolver,
	scopeType string,
	targets []string,
) (map[string]policies.ResolvedApp, error) {
	if scopeType != plans.ScopeTypeApplications || len(targets) == constants.DefaultInitValue {
		return nil, nil
	}
	resolved, missing, err := resolveApps(ctx, targets)
	if err != nil {
		return nil, err
	}
	if len(missing) > constants.DefaultInitValue {
		return nil, validation.Invalidf(string(ErrMissingApplications), missing)
	}
	return resolved, nil
}

func updatable(phase string) bool {
	return phase == plans.PhaseActive ||
		phase == plans.PhaseScheduled ||
		phase == plans.PhasePendingApproval ||
		phase == plans.PhaseFailed ||
		phase == plans.PhaseDraft
}

func newTargets(scope planseps.ScopeRequest) []string {
	if scope.Type == plans.ScopeTypeApplications {
		return append([]string(nil), scope.ApplicationIDs...)
	}
	return append([]string(nil), scope.Namespaces...)
}

func scopeTargets(plan *plans.ProtectionPlan) []string {
	if plan.Scope.Type == plans.ScopeTypeApplications {
		return append([]string(nil), plan.Scope.ApplicationIDs...)
	}
	return append([]string(nil), plan.Scope.Namespaces...)
}

func allTargets(diff TargetDiff) []string {
	out := make([]string, constants.DefaultInitValue, len(diff.Added)+len(diff.Removed)+len(diff.Unchanged))
	out = append(out, diff.Added...)
	out = append(out, diff.Removed...)
	out = append(out, diff.Unchanged...)
	return out
}

func toPolicies(items []planseps.PolicyRequest) []plans.ProtectionPlanPolicy {
	out := make([]plans.ProtectionPlanPolicy, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, plans.ProtectionPlanPolicy{TemplateID: item.TemplateID, Params: item.Params})
	}
	return out
}
