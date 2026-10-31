package duplicate

import (
	"fmt"
	"strings"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
)

const (
	namePrefix      = "Copy of "
	nameSuffixFmt   = "%s %d"
	firstNameSuffix = 2
	maxNameAttempts = 50
	// " " plus the two digits of the last suffix tried.
	maxSuffixRunes = 3
)

const errNoFreeName errors.Error = "could not find an available name for a copy of %q"

func UsesDefaultName(overrides planseps.DuplicateProtectionPlanRequest) bool {
	return overrides.Name == nil || *overrides.Name == constants.EmptyString
}

// The default copy name collides as soon as a plan is duplicated twice, so it is suffixed
// until free rather than rejected.
func AvailableName(base string, existing []plans.ProtectionPlan) (string, error) {
	if validation.UniqueName(existing, base, constants.EmptyString) == nil {
		return base, nil
	}
	short := fitName(base, validation.NameMaxLength-maxSuffixRunes)
	for suffix := firstNameSuffix; suffix < firstNameSuffix+maxNameAttempts; suffix++ {
		candidate := fmt.Sprintf(nameSuffixFmt, short, suffix)
		if validation.UniqueName(existing, candidate, constants.EmptyString) == nil {
			return candidate, nil
		}
	}
	return constants.EmptyString, validation.Invalidf(string(errNoFreeName), base)
}

// Prepare-shaped on purpose: callers feed the result back into the prepare flow so duplication
// reuses the standard validation and rendering pipeline.
func BuildRequest(
	source *plans.ProtectionPlan,
	overrides planseps.DuplicateProtectionPlanRequest,
	callerOwner bool,
) *planseps.PrepareProtectionPlanRequest {
	timeMode := source.TimeMode
	if overrides.TimeMode != nil {
		timeMode = *overrides.TimeMode
	}
	env := source.EnvironmentRef
	if overrides.EnvironmentRef != nil {
		env = *overrides.EnvironmentRef
	}
	kind := source.KindRef
	if overrides.KindRef != nil {
		kind = *overrides.KindRef
	}
	tags := source.TagRefs
	if overrides.TagRefs != nil {
		tags = overrides.TagRefs
	}
	return &planseps.PrepareProtectionPlanRequest{
		ApprovalMode:    resolveApprovalMode(source, overrides, callerOwner),
		Name:            resolveName(source.Name, overrides.Name),
		Description:     source.Description,
		Severity:        source.Severity,
		Priority:        source.Priority,
		Scope:           toScopeRequest(source.Scope),
		Policies:        toPolicyRequests(source.Policies),
		Mode:            source.Mode,
		TimeMode:        timeMode,
		TimeRange:       resolveTimeRange(timeMode, overrides.TimeRange, source.TimeRange),
		Repeat:          resolveRepeat(timeMode, overrides.Repeat, source.Repeat),
		ParticipantRefs: source.ParticipantRefs,
		EnvironmentRef:  &env,
		KindRef:         &kind,
		TagRefs:         tags,
	}
}

func resolveRepeat(timeMode string, override, source *plans.ProtectionPlanRepeat) *plans.ProtectionPlanRepeat {
	if timeMode != plans.TimeModeRepeating {
		return nil
	}
	if override != nil {
		return override
	}
	return source
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

// The UI sends environmentRef whenever tags are touched, so "sent" is not "changed";
// nil hands the derivation back to Prepare. Only an Owner may relax a required source.
func resolveApprovalMode(
	source *plans.ProtectionPlan,
	overrides planseps.DuplicateProtectionPlanRequest,
	callerOwner bool,
) *string {
	if source.ApprovalMode == plans.ApprovalModeRequired && !callerOwner {
		return &source.ApprovalMode
	}
	if overrides.ApprovalMode != nil {
		return overrides.ApprovalMode
	}
	if source.ApprovalMode == constants.EmptyString {
		return nil
	}
	if overrides.EnvironmentRef != nil && *overrides.EnvironmentRef != source.EnvironmentRef {
		return nil
	}
	return &source.ApprovalMode
}

func resolveName(sourceName string, override *string) string {
	if override != nil && *override != constants.EmptyString {
		return *override
	}
	return fitName(namePrefix+sourceName, validation.NameMaxLength)
}

// A long source name would push the default copy name past the limit, so it is cut instead of refused.
func fitName(name string, limit int) string {
	runes := []rune(name)
	if len(runes) <= limit {
		return name
	}
	return strings.TrimSpace(string(runes[:limit]))
}

func toScopeRequest(scope plans.ProtectionPlanScope) planseps.ScopeRequest {
	return planseps.ScopeRequest{
		Type:            scope.Type,
		ApplicationRefs: scope.ApplicationRefs,
		Namespaces:      scope.Namespaces,
		Exclusions:      plans.NormalizeExclusions(scope.Exclusions),
	}
}

func toPolicyRequests(items []plans.ProtectionPlanPolicy) []planseps.PolicyRequest {
	out := make([]planseps.PolicyRequest, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, planseps.PolicyRequest{TemplateID: item.TemplateID, Params: item.Params})
	}
	return out
}
