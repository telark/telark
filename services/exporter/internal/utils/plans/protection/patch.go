package protection

import (
	"errors"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func ValidatePatchScope(body map[string]any) error {
	raw, ok := body[constants.FieldScope]
	if !ok {
		return nil
	}
	scope, ok := raw.(map[string]any)
	if !ok {
		return errors.New(string(constants.ErrProtectionPlanInvalidScope))
	}

	scopeType, ok := scope[constants.FieldScopeType].(string)
	if !ok {
		return errors.New(string(constants.ErrProtectionPlanInvalidScope))
	}
	apps := stringSlice(scope[constants.FieldScopeAppRefs])
	namespaces := stringSlice(scope[constants.FieldScopeNamespaces])
	return validateScope(scopeType, len(apps), len(namespaces), hasExclusionResources(scope))
}

// Anything but an object is left to the CRD schema, which also refuses a null on a repeating plan.
func ValidatePatchRepeat(body map[string]any) error {
	raw, ok := body[constants.FieldRepeat].(map[string]any)
	if !ok {
		return nil
	}
	repeat, err := sharedutils.ExtractStructFromBody[plans.ProtectionPlanRepeat](raw)
	if err != nil {
		return err
	}
	return plans.ValidateRepeat(repeat)
}

// stringSlice is not reusable here: resources are maps, so it would report none.
func hasExclusionResources(scope map[string]any) bool {
	excl, ok := scope[constants.FieldScopeExclusions].(map[string]any)
	if !ok {
		return false
	}
	resources, ok := excl[constants.FieldExclusionResources].([]any)
	return ok && len(resources) > constants.DefaultInitValue
}

func stringSlice(v any) []string {
	switch cast := v.(type) {
	case []string:
		return cast
	case []any:
		out := make([]string, constants.DefaultInitValue, len(cast))
		for _, item := range cast {
			s, ok := item.(string)
			if !ok {
				return nil
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}
