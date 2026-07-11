package protection

import (
	"errors"

	"github.com/telark/data/plans"
	"github.com/telark/exporter/constants"
)

func Validate(plan *plans.ProtectionPlan) error {
	if plan == nil {
		return errors.New(string(constants.ErrProtectionPlanIDRequired))
	}
	if plan.ID == constants.EmptyString {
		return errors.New(string(constants.ErrProtectionPlanIDRequired))
	}
	if plan.Name == constants.EmptyString {
		return errors.New(string(constants.ErrProtectionPlanNameRequired))
	}
	if len(plan.Policies) == constants.DefaultInitValue {
		return errors.New(string(constants.ErrProtectionPlanPoliciesRequired))
	}
	return validateScope(plan.Scope)
}

func validateScope(scope plans.ProtectionPlanScope) error {
	switch scope.Type {
	case constants.ScopeTypeApplications:
		if len(scope.ApplicationIDs) == constants.DefaultInitValue || len(scope.Namespaces) > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
		return nil
	case constants.ScopeTypeNamespaces:
		if len(scope.Namespaces) == constants.DefaultInitValue || len(scope.ApplicationIDs) > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
		return nil
	default:
		return errors.New(string(constants.ErrProtectionPlanInvalidScope))
	}
}
