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
	policyDiff := diffPolicies(plan.Policies, newPolicies)
	targetDiff := diffTargets(scopeTargets(plan), newTargets(req.Scope))

	resolved, err := resolveTargets(ctx, deps.ResolveApps, plan.Scope.Type, allTargets(targetDiff))
	if err != nil {
		return nil, err
	}

	deployCombos := append(
		protpolicies.Combinations(newPolicies, targetDiff.Added),
		protpolicies.Combinations(policyDiff.Added, targetDiff.Unchanged)...,
	)
	removeCombos := append(
		protpolicies.Combinations(plan.Policies, targetDiff.Removed),
		protpolicies.Combinations(policyDiff.Removed, targetDiff.Unchanged)...,
	)

	deployed, kept, err := protpolicies.ApplyClusterDiff(
		ctx, deps.Applier, deps.Logger, plan,
		deployCombos, removeCombos, resolved, req.Mode,
		protpolicies.DeployErrorMessages{
			GenericDeployFailure: deployFailureGeneric,
			InternalErrorFormat:  string(ErrInternal),
		},
	)
	if err != nil {
		return nil, err
	}

	rendered := protpolicies.UnionRenderedNames(kept, deployed)
	now := deps.Clock().Format(globalshared.DefaultTimeFormat)
	patch, changed := BuildPatch(plan, req, newPolicies, rendered, userID, now)
	if !changed {
		return plan, nil
	}

	if err := deps.Exporter.PatchOrError(userID, planID, patch); err != nil {
		protpolicies.RollbackPatchFailure(ctx, deps.Applier, deps.Logger, plan, deployed)
		return nil, fmt.Errorf(string(ErrPartial), planID)
	}
	if plan.Phase == plans.PhaseActive && deps.StampHealth != nil {
		deps.StampHealth(planID)
	}
	return deps.Exporter.Get(planID)
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
	if err := validation.Fields(req); err != nil {
		return err
	}
	if err := ensureNameAvailable(deps, plan, req.Name); err != nil {
		return err
	}
	if err := validation.NamespaceScope(ctx, req.Scope.Type, req.Scope.Namespaces, deps.ListNamespaces); err != nil {
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
