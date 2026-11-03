package protection

import (
	"errors"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/text"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func Validate(plan *plans.ProtectionPlan) error {
	if plan == nil || plan.ID == constants.EmptyString {
		return errors.New(string(constants.ErrProtectionPlanIDRequired))
	}
	if plan.Name == constants.EmptyString {
		return errors.New(string(constants.ErrProtectionPlanNameRequired))
	}
	if len(plan.Policies) == constants.DefaultInitValue {
		return errors.New(string(constants.ErrProtectionPlanPoliciesRequired))
	}
	if plan.TimeMode == plans.TimeModeRepeating {
		if err := plans.ValidateRepeat(plan.Repeat); err != nil {
			return err
		}
	}
	scope := plan.Scope
	hasExclusionResources := scope.Exclusions != nil && len(scope.Exclusions.Resources) > constants.DefaultInitValue
	return validateScope(scope.Type, len(scope.ApplicationRefs), len(scope.Namespaces), hasExclusionResources)
}

func validateScope(scopeType string, appCount, namespaceCount int, hasExclusionResources bool) error {
	switch scopeType {
	case constants.ScopeTypeApplications:
		if appCount == constants.DefaultInitValue || namespaceCount > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
		return nil
	case constants.ScopeTypeNamespaces:
		if namespaceCount == constants.DefaultInitValue || appCount > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
		if hasExclusionResources {
			return errors.New(string(constants.ErrProtectionPlanExclusionScope))
		}
		return nil
	default:
		return errors.New(string(constants.ErrProtectionPlanInvalidScope))
	}
}

// A failure reason carries upstream error text, so it is cut to the schema's limit
// rather than failing the status write that records the failure.
func TruncateReason(body map[string]any) {
	if reason, isString := body[constants.FieldPlanReason].(string); isString {
		body[constants.FieldPlanReason] = text.Truncate(reason, constants.MaxPlanReasonLength)
	}
}
