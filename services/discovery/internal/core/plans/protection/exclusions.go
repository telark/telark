package protection

import (
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
)

// Both arrays are always present because merge patch replaces arrays but merges objects, and nil
// becomes JSON null so merge patch deletes the key: a stored CR never carries an empty object.
func ExclusionsPatchValue(e *plans.ProtectionPlanScopeExclusions) any {
	e = plans.NormalizeExclusions(e)
	if e == nil {
		return nil
	}
	kinds := make([]any, constants.DefaultInitValue, len(e.Kinds))
	for _, kind := range e.Kinds {
		kinds = append(kinds, kind)
	}
	resources := make([]any, constants.DefaultInitValue, len(e.Resources))
	for _, r := range e.Resources {
		resources = append(resources, map[string]any{
			FieldExclusionKind:      r.Kind,
			FieldExclusionName:      r.Name,
			FieldExclusionNamespace: r.Namespace,
		})
	}
	return map[string]any{
		FieldExclusionKinds:     kinds,
		FieldExclusionResources: resources,
	}
}
