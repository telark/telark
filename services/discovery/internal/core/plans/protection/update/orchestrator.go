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
	planseps "github.com/telark/rest/endpoints/plans"
)

type (
	NamespaceLister func(ctx context.Context) ([]string, error)
	Deps            struct {
		Applier        *protpolicies.Applier
		Exporter       *clients.ProtectionPlanClient
		ResolveApps    applications.Resolver
		ListNamespaces NamespaceLister
		Logger         protection.Logger
		Clock          func() time.Time
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
	patch, changed := buildPatch(plan, req, newPolicies, rendered, userID, now)
	if !changed {
		return plan, nil
	}

	if err := deps.Exporter.PatchOrError(userID, planID, patch); err != nil {
		protpolicies.RollbackPatchFailure(ctx, deps.Applier, deps.Logger, plan, deployed)
		return nil, fmt.Errorf(string(ErrPartial), planID)
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
		return fmt.Errorf(fmtRawString, ErrInvalidPhase)
	}
	if req.Scope.Type != plan.Scope.Type {
		return fmt.Errorf(fmtRawString, ErrScopeTypeChange)
	}
	if err := validateScopeTargets(ctx, deps, req.Scope); err != nil {
		return err
	}
	if err := validatePolicies(req.Policies, req.Scope.Type); err != nil {
		return fmt.Errorf(string(ErrInvalidPolicies), err)
	}
	if req.TimeMode == plans.TimeModeTimeRange {
		if err := validateTimeRange(req.TimeRange); err != nil {
			return err
		}
	}
	return nil
}

func validateScopeTargets(ctx context.Context, deps Deps, scope planseps.ScopeRequest) error {
	if scope.Type == plans.ScopeTypeNamespaces && deps.ListNamespaces != nil {
		existing, err := deps.ListNamespaces(ctx)
		if err != nil {
			return err
		}
		set := stringSet(existing)
		var missing []string
		for _, ns := range scope.Namespaces {
			if _, ok := set[ns]; !ok {
				missing = append(missing, ns)
			}
		}
		if len(missing) > constants.DefaultInitValue {
			return fmt.Errorf(string(ErrMissingNamespaces), missing)
		}
	}
	return nil
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
		return nil, fmt.Errorf(string(ErrMissingApplications), missing)
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
