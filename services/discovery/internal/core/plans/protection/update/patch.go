package update

import (
	"github.com/telark/data/plans"
	planseps "github.com/telark/rest/endpoints/plans"
)

// Only changed fields plus the recomputed renderedPolicies/health/lastUpdated; phase, reason
// and the started/terminated stamps belong to the lifecycle handlers and are never set here.
func BuildPatch(
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
	rendered []string,
	userID, now string,
) (planseps.PatchProtectionPlanRequest, bool) {
	healthUnknown := plans.HealthUnknown
	patch := planseps.PatchProtectionPlanRequest{
		LastUpdatedAt:    &now,
		LastUpdatedBy:    &userID,
		Health:           &healthUnknown,
		RenderedPolicies: rendered,
	}
	scalarChanged := applyScalarPatch(&patch, plan, req)
	complexChanged := applyComplexPatch(&patch, plan, req, newPolicies)
	renderedChanged := !stringSliceSetEqual(plan.RenderedPolicies, rendered)
	return patch, scalarChanged || complexChanged || renderedChanged
}

func applyScalarPatch(
	patch *planseps.PatchProtectionPlanRequest,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
) bool {
	changed := false
	if plan.Name != req.Name {
		patch.Name = &req.Name
		changed = true
	}
	if !stringPtrEqual(plan.Description, req.Description) {
		patch.Description = req.Description
		changed = true
	}
	if plan.Severity != req.Severity {
		patch.Severity = &req.Severity
		changed = true
	}
	if plan.Priority != req.Priority {
		priority := req.Priority
		patch.Priority = &priority
		changed = true
	}
	if plan.Mode != req.Mode {
		patch.Mode = &req.Mode
		changed = true
	}
	if plan.TimeMode != req.TimeMode {
		patch.TimeMode = &req.TimeMode
		changed = true
	}
	if req.EnvironmentRef != nil && plan.EnvironmentRef != *req.EnvironmentRef {
		patch.EnvironmentRef = req.EnvironmentRef
		changed = true
	}
	return changed
}

func applyComplexPatch(
	patch *planseps.PatchProtectionPlanRequest,
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
) bool {
	changed := false
	if !timeRangeEqual(plan.TimeRange, req.TimeRange) {
		patch.TimeRange = toPatchTimeRange(req.TimeRange)
		changed = true
	}
	if !scopeTargetsEqual(plan.Scope, req.Scope) || exclusionsChanged(plan.Scope, req.Scope) {
		patch.Scope = &planseps.ScopeRequest{
			Type:            req.Scope.Type,
			ApplicationRefs: req.Scope.ApplicationRefs,
			Namespaces:      req.Scope.Namespaces,
			Exclusions:      effectiveExclusions(plan.Scope, req.Scope),
		}
		changed = true
	}
	if !planPoliciesEqual(plan.Policies, newPolicies) {
		patch.Policies = req.Policies
		changed = true
	}
	if !stringSliceSetEqual(plan.ParticipantRefs, req.ParticipantRefs) {
		patch.ParticipantRefs = req.ParticipantRefs
		changed = true
	}
	if req.TagRefs != nil && !stringSliceSetEqual(plan.TagRefs, req.TagRefs) {
		tags := req.TagRefs
		patch.TagRefs = &tags
		changed = true
	}
	return changed
}

// Material = anything that changes what gets rendered or when; kept beside the comparators so they cannot drift.
func MaterialChange(
	plan *plans.ProtectionPlan,
	req *planseps.PrepareProtectionPlanRequest,
	newPolicies []plans.ProtectionPlanPolicy,
) bool {
	return !planPoliciesEqual(plan.Policies, newPolicies) ||
		!scopeTargetsEqual(plan.Scope, req.Scope) ||
		plan.Mode != req.Mode ||
		plan.TimeMode != req.TimeMode ||
		!timeRangeEqual(plan.TimeRange, req.TimeRange) ||
		exclusionsChanged(plan.Scope, req.Scope)
}

// Nil request exclusions mean untouched, like TagRefs; a non-nil value replaces them whole.
func exclusionsChanged(planScope plans.ProtectionPlanScope, reqScope planseps.ScopeRequest) bool {
	return reqScope.Exclusions != nil && !plans.ExclusionsEqual(planScope.Exclusions, reqScope.Exclusions)
}

func effectiveExclusions(planScope plans.ProtectionPlanScope, reqScope planseps.ScopeRequest) *plans.ProtectionPlanScopeExclusions {
	if reqScope.Exclusions != nil {
		return plans.NormalizeExclusions(reqScope.Exclusions)
	}
	return planScope.Exclusions
}

func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func timeRangeEqual(a *plans.ProtectionPlanTimeRange, b *planseps.TimeRangeRequest) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.StartAt == b.StartAt && a.EndAt == b.EndAt
}

func toPatchTimeRange(tr *planseps.TimeRangeRequest) *planseps.TimeRangeRequest {
	if tr == nil {
		return nil
	}
	dup := *tr
	return &dup
}

func scopeTargetsEqual(planScope plans.ProtectionPlanScope, reqScope planseps.ScopeRequest) bool {
	if planScope.Type == plans.ScopeTypeApplications {
		return stringSliceSetEqual(planScope.ApplicationRefs, reqScope.ApplicationRefs)
	}
	return stringSliceSetEqual(planScope.Namespaces, reqScope.Namespaces)
}

func planPoliciesEqual(a, b []plans.ProtectionPlanPolicy) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := make(map[PolicyKey]struct{}, len(a))
	for _, p := range a {
		aSet[policyKey(p)] = struct{}{}
	}
	for _, p := range b {
		if _, ok := aSet[policyKey(p)]; !ok {
			return false
		}
	}
	return true
}

func stringSliceSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aSet := stringSet(a)
	for _, v := range b {
		if _, ok := aSet[v]; !ok {
			return false
		}
	}
	return true
}
