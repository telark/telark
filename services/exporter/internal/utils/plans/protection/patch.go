package protection

import (
	"errors"

	"github.com/telark/exporter/internal/constants"
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
	apps := stringSlice(scope[constants.FieldScopeAppIDs])
	namespaces := stringSlice(scope[constants.FieldScopeNamespaces])

	switch scopeType {
	case constants.ScopeTypeApplications:
		if len(apps) == constants.DefaultInitValue || len(namespaces) > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
	case constants.ScopeTypeNamespaces:
		if len(namespaces) == constants.DefaultInitValue || len(apps) > constants.DefaultInitValue {
			return errors.New(string(constants.ErrProtectionPlanScopeUnion))
		}
	default:
		return errors.New(string(constants.ErrProtectionPlanInvalidScope))
	}
	return nil
}

func stringSlice(v any) []string {
	if v == nil {
		return nil
	}
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
