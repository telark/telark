package plans

import (
	"cmp"
	"slices"

	"github.com/telark/data/constants"
)

// NormalizeExclusions returns a sorted, de-duplicated copy that never aliases e, or nil when
// there is nothing to exclude, so absent and empty exclusions compare and render the same.
func NormalizeExclusions(e *ProtectionPlanScopeExclusions) *ProtectionPlanScopeExclusions {
	if e == nil || len(e.Kinds)+len(e.Resources) == constants.DefaultInitValue {
		return nil
	}
	kinds := slices.Clone(e.Kinds)
	slices.Sort(kinds)
	resources := slices.Clone(e.Resources)
	slices.SortFunc(resources, func(a, b ProtectionPlanExcludedResource) int {
		return cmp.Or(
			cmp.Compare(a.Namespace, b.Namespace),
			cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name),
		)
	})
	return &ProtectionPlanScopeExclusions{
		Kinds:     slices.Compact(kinds),
		Resources: slices.Compact(resources),
	}
}

func ExclusionsEqual(a, b *ProtectionPlanScopeExclusions) bool {
	na, nb := NormalizeExclusions(a), NormalizeExclusions(b)
	if na == nil || nb == nil {
		return na == nb
	}
	return slices.Equal(na.Kinds, nb.Kinds) && slices.Equal(na.Resources, nb.Resources)
}
