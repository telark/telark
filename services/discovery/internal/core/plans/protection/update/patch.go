package update

import (
	"github.com/telark/data/plans"
	planseps "github.com/telark/rest/endpoints/plans"
)

// BuildPatch produces the exporter PATCH request limited to fields that actually changed,
// plus the always-recomputed renderedPolicies/health/lastUpdated bookkeeping. It NEVER sets
// id, createdAt/By, phase, startedAt/By, terminatedAt/By, or reason — those are owned by the
// lifecycle handlers (cancel, reactivate, activate, terminate).
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
	if req.EnvironmentID != nil && plan.EnvironmentID != *req.EnvironmentID {
		patch.EnvironmentID = req.EnvironmentID
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
	if !scopeTargetsEqual(plan.Scope, req.Scope) {
		patch.Scope = &planseps.ScopeRequest{
			Type:           req.Scope.Type,
			ApplicationIDs: req.Scope.ApplicationIDs,
			Namespaces:     req.Scope.Namespaces,
		}
		changed = true
	}
	if !planPoliciesEqual(plan.Policies, newPolicies) {
		patch.Policies = req.Policies
		changed = true
	}
	if !stringSliceSetEqual(plan.ParticipantsIDs, req.ParticipantsIDs) {
		patch.ParticipantsIDs = req.ParticipantsIDs
		changed = true
	}
	if req.TagIDs != nil && !stringSliceSetEqual(plan.TagIDs, req.TagIDs) {
		tags := req.TagIDs
		patch.TagIDs = &tags
		changed = true
	}
	return changed
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
		return stringSliceSetEqual(planScope.ApplicationIDs, reqScope.ApplicationIDs)
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
