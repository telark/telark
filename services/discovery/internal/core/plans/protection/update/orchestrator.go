package update

import (
	"context"
	"fmt"
	"maps"
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
		Environments   validation.EnvironmentLister
		Logger         protection.Logger
		Clock          func() time.Time
		// The patch blanks health; this restamps it without waiting for a controller tick.
		StampHealth func(planID string)
		// Called after a material edit of a pending plan re-requests approval.
		NotifyApprovers func(*plans.ProtectionPlan)
		// Serializes a rename with parallel creates of the same name.
		LockName func(ctx context.Context, name string) (func(), error)
		// Deploys a scheduled plan whose edited window starts now; the controller's activate path.
		Activate func(ctx context.Context, plan *plans.ProtectionPlan) error
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
	clock := deps.Clock()
	if err := validateRequest(ctx, plan, req, deps, clock); err != nil {
		return nil, err
	}
	release, err := lockRename(ctx, deps, plan, req.Name)
	if err != nil {
		return nil, err
	}
	defer release()
	if err := ensureNameAvailable(deps, plan, req.Name); err != nil {
		return nil, err
	}

	newPolicies := toPolicies(req.Policies)
	if protection.RequiresApproval(plan) && approvedPhase(plan.Phase) && MaterialChange(plan, req, newPolicies) {
		return nil, validation.Invalid(string(protection.ErrApprovedPlanMaterialEdit))
	}
	return apply(ctx, deps, userID, plan, req, newPolicies, clock)
}

func apply(
	ctx context.Context,
	deps Deps,
	userID string,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
	clock time.Time,
) (*plans.ProtectionPlan, error) {
	// The plan name is baked into every policy annotation.
	fullRender := exclusionsChanged(plan.Scope, req.Scope) || plan.Name != req.Name
	targetDiff := diffTargets(scopeTargets(plan), newTargets(req.Scope))
	resolved, err := resolveTargets(ctx, deps.ResolveApps, plan.Scope.Type, allTargets(targetDiff))
	if err != nil {
		return nil, err
	}

	park := plan.Phase == plans.PhaseActive && TargetPhase(plan, req, clock) == plans.PhaseScheduled
	deployed, kept, stale, err := applyCluster(ctx, deps, plan, req, newPolicies, targetDiff, resolved, fullRender, park)
	if err != nil {
		return nil, err
	}

	rendered := protpolicies.UnionRenderedNames(kept, deployed)
	now := clock.Format(globalshared.DefaultTimeFormat)
	patch, changed := BuildPatch(plan, req, newPolicies, rendered, userID, now)
	if !changed && !park {
		return plan, nil
	}

	material := MaterialChange(plan, req, newPolicies)
	reRequest := plan.Phase == plans.PhasePendingApproval && material
	if err := applyPatch(deps, plan, userID, now, patch, reRequest || recordsEditor(plan, material), park); err != nil {
		rollbackCluster(ctx, deps, plan, deployed, resolved, fullRender)
		return nil, fmt.Errorf(string(ErrPartial), plan.ID)
	}
	if err := deps.Applier.DeletePoliciesByLabelAndNames(ctx, plan.ID, stale); err != nil {
		deps.Logger.Error(fmt.Sprintf(protection.LogStaleDeleteFailed, plan.ID, err))
	}
	if park {
		plan.Phase = plans.PhaseScheduled
	}
	return settlePhase(ctx, deps, plan, req, clock, reRequest)
}

// A scheduled plan whose edited window starts now goes through the controller's activate path.
func settlePhase(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	now time.Time,
	reRequest bool,
) (*plans.ProtectionPlan, error) {
	if plan.Phase == plans.PhaseScheduled && TargetPhase(plan, req, now) == plans.PhaseActive {
		if err := deps.Activate(ctx, plan); err != nil {
			return nil, err
		}
	}
	return afterPatch(deps, plan, reRequest)
}

// The phase the edited window puts a scheduled or active plan in; other phases keep their own.
// An unchanged elapsed window is left to the controller to terminate.
func TargetPhase(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest, now time.Time) string {
	if plan.Phase != plans.PhaseActive && plan.Phase != plans.PhaseScheduled {
		return plan.Phase
	}
	target := *plan
	target.TimeMode = req.TimeMode
	target.TimeRange = nil
	if req.TimeRange != nil {
		target.TimeRange = &plans.ProtectionPlanTimeRange{StartAt: req.TimeRange.StartAt, EndAt: req.TimeRange.EndAt}
	}
	phase, expired := protection.WindowPhase(&target, now)
	if expired {
		return plan.Phase
	}
	return phase
}

func lockRename(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	name string,
) (func(), error) {
	if deps.LockName == nil || validation.NormalizeName(name) == validation.NormalizeName(plan.Name) {
		return func() {}, nil
	}
	return deps.LockName(ctx, name)
}

// An exclusions change is content under the same names, which the combos diff never re-renders,
// so it deploys the whole new plan instead. Parking withdraws everything: the plan enforces
// nothing until its new window starts.
func applyCluster(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
	targetDiff TargetDiff,
	resolved map[string]policies.ResolvedApp,
	fullRender, park bool,
) (deployed, kept, stale []string, err error) {
	if park {
		if err := deps.Applier.CleanupByPlanID(ctx, plan.ID); err != nil {
			deps.Logger.Error(fmt.Sprintf(protection.LogCleanupFailed, stagePark, plan.ID, err))
		}
		return nil, nil, nil, nil
	}
	msgs := protpolicies.DeployErrorMessages{
		GenericDeployFailure: deployFailureGeneric,
		InternalErrorFormat:  string(ErrInternal),
	}
	if fullRender {
		return protpolicies.ApplyFullRender(ctx, deps.Applier, deps.Logger, plan, RenderTarget(plan, req, newPolicies), resolved, msgs)
	}
	policyDiff := diffPolicies(plan.Policies, newPolicies)
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

func RenderTarget(
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
) *plans.ProtectionPlan {
	target := *plan
	target.Name = req.Name
	target.Scope = plans.ProtectionPlanScope{
		Type:            plan.Scope.Type,
		ApplicationRefs: req.Scope.ApplicationRefs,
		Namespaces:      req.Scope.Namespaces,
		Exclusions:      effectiveExclusions(plan.Scope, req.Scope),
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

// A material edit of a pending plan resets the approval request in the same patch, so an
// approver can never act on a version they did not see; of any other unapproved required plan
// it records the editor, who then cannot approve its reactivation.
func applyPatch(
	deps Deps,
	plan *plans.ProtectionPlan,
	userID, now string,
	patch planseps.PatchProtectionPlanRequest,
	approvalTouched, park bool,
) error {
	if !approvalTouched && !park && patch.Scope == nil {
		return deps.Exporter.PatchOrError(userID, plan.ID, patch)
	}
	body, err := restmapper.MapToJSONPayload(patch)
	if err != nil {
		return err
	}
	if scope, ok := body[protection.FieldScope].(map[string]any); ok && patch.Scope != nil {
		scope[protection.FieldScopeExclusions] = protection.ExclusionsPatchValue(patch.Scope.Exclusions)
	}
	switch {
	case !approvalTouched:
	case plan.Phase == plans.PhasePendingApproval:
		body[protection.FieldApproval] = protection.ApprovalPatchValue(protection.NewApprovalRequest(userID, now, plan.Approval))
	default:
		body[protection.FieldApproval] = protection.ApprovalPatchValue(protection.RecordEditor(plan.Approval, userID, now))
	}
	if park {
		maps.Copy(body, protection.BuildScheduledPatch(userID, now))
	}
	return deps.Exporter.PatchRawOrError(userID, plan.ID, body)
}

// Moving an automatic plan into an environment that derives required would label it with a
// gate it never passed.
func EnvironmentRaisesApproval(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest) bool {
	return req.EnvironmentRef != nil && *req.EnvironmentRef != plan.EnvironmentRef && !protection.RequiresApproval(plan) &&
		protection.DerivedApprovalMode(*req.EnvironmentRef) == plans.ApprovalModeRequired
}

func ApprovalModeChanged(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest) bool {
	return req.ApprovalMode != nil &&
		protection.EffectiveApprovalMode(*req.ApprovalMode) != protection.EffectiveApprovalMode(plan.ApprovalMode)
}

func recordsEditor(plan *plans.ProtectionPlan, material bool) bool {
	return material && plan.Approval != nil && protection.RequiresApproval(plan) &&
		plan.Phase != plans.PhasePendingApproval && !approvedPhase(plan.Phase)
}

func approvedPhase(phase string) bool {
	return phase == plans.PhaseActive || phase == plans.PhaseScheduled
}

// Every phase is editable: a canceled or terminated plan has no cluster state, so the edit only
// re-renders once it is reactivated.
func validateRequest(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	deps Deps,
	now time.Time,
) error {
	if req.Scope.Type != plan.Scope.Type {
		return validation.Invalidf(fmtRawString, ErrScopeTypeChange)
	}
	if ApprovalModeChanged(plan, req) || EnvironmentRaisesApproval(plan, req) {
		return validation.Invalid(string(protection.ErrApprovalModeImmutable))
	}
	validation.Normalize(req)
	if err := validation.Fields(req); err != nil {
		return err
	}
	if err := callerFields(ctx, req, deps); err != nil {
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
	if req.TimeMode != plans.TimeModeTimeRange {
		return nil
	}
	// Only a newly set window has to be open: a finished plan can still be renamed as it is.
	if plan.TimeMode != req.TimeMode || !timeRangeEqual(plan.TimeRange, req.TimeRange) {
		return validation.TimeRangeOpen(req.TimeRange, now)
	}
	return validation.TimeRange(req.TimeRange)
}

func callerFields(ctx context.Context, req *planseps.PrepareProtectionPlanRequest, deps Deps) error {
	if err := validation.EnforceScope(ctx, req.Scope.Type, req.Mode); err != nil {
		return err
	}
	return validation.EnvironmentRef(req.EnvironmentRef, deps.Environments)
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

func newTargets(scope planseps.ScopeRequest) []string {
	if scope.Type == plans.ScopeTypeApplications {
		return append([]string(nil), scope.ApplicationRefs...)
	}
	return append([]string(nil), scope.Namespaces...)
}

func scopeTargets(plan *plans.ProtectionPlan) []string {
	if plan.Scope.Type == plans.ScopeTypeApplications {
		return append([]string(nil), plan.Scope.ApplicationRefs...)
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
