package duplicate

import (
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	planseps "github.com/telark/rest/endpoints/plans"
)

const namePrefix = "Copy of "

// Prepare-shaped on purpose: callers feed the result back into the prepare flow so duplication
// reuses the standard validation and rendering pipeline.
func BuildRequest(
	source *plans.ProtectionPlan,
	overrides planseps.DuplicateProtectionPlanRequest,
) *planseps.PrepareProtectionPlanRequest {
	timeMode := source.TimeMode
	if overrides.TimeMode != nil {
		timeMode = *overrides.TimeMode
	}
	return &planseps.PrepareProtectionPlanRequest{
		Name:            resolveName(source.Name, overrides.Name),
		Description:     source.Description,
		Severity:        source.Severity,
		Priority:        source.Priority,
		Scope:           toScopeRequest(source.Scope),
		Policies:        toPolicyRequests(source.Policies),
		Mode:            source.Mode,
		TimeMode:        timeMode,
		TimeRange:       resolveTimeRange(timeMode, overrides.TimeRange, source.TimeRange),
		ParticipantsIDs: source.ParticipantsIDs,
	}
}

func resolveTimeRange(
	timeMode string,
	override *planseps.TimeRangeRequest,
	source *plans.ProtectionPlanTimeRange,
) *planseps.TimeRangeRequest {
	if timeMode != plans.TimeModeTimeRange {
		return nil
	}
	if override != nil {
		return &planseps.TimeRangeRequest{StartAt: override.StartAt, EndAt: override.EndAt}
	}
	if source == nil {
		return nil
	}
	return &planseps.TimeRangeRequest{StartAt: source.StartAt, EndAt: source.EndAt}
}

func resolveName(sourceName string, override *string) string {
	if override != nil && *override != constants.EmptyString {
		return *override
	}
	return namePrefix + sourceName
}

func toScopeRequest(scope plans.ProtectionPlanScope) planseps.ScopeRequest {
	return planseps.ScopeRequest{
		Type:           scope.Type,
		ApplicationIDs: scope.ApplicationIDs,
		Namespaces:     scope.Namespaces,
	}
}

func toPolicyRequests(items []plans.ProtectionPlanPolicy) []planseps.PolicyRequest {
	out := make([]planseps.PolicyRequest, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, planseps.PolicyRequest{TemplateID: item.TemplateID, Params: item.Params})
	}
	return out
}
