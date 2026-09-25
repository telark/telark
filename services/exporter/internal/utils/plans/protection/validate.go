package protection

import (
	"errors"

	"github.com/telark/data/plans"
	"github.com/telark/exporter/internal/constants"
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
	scope := plan.Scope
	hasExclusionResources := scope.Exclusions != nil && len(scope.Exclusions.Resources) > constants.DefaultInitValue
	return validateScope(scope.Type, len(scope.ApplicationIDs), len(scope.Namespaces), hasExclusionResources)
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
