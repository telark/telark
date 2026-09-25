package policies

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
)

// Resources outside the namespace are dropped: a namespaced Policy cannot reach them anyway.
func ExclusionFilters(excl *plans.ProtectionPlanScopeExclusions, namespace string) kyvernov1.ResourceFilters {
	norm := plans.NormalizeExclusions(excl)
	if norm == nil {
		return nil
	}
	out := make(kyvernov1.ResourceFilters, constants.DefaultInitValue, len(norm.Kinds)+len(norm.Resources))
	for _, kind := range norm.Kinds {
		out = append(out, kyvernov1.ResourceFilter{
			ResourceDescription: kyvernov1.ResourceDescription{Kinds: expandExcludedKind(kind)},
		})
	}
	for _, r := range norm.Resources {
		if r.Namespace != namespace {
			continue
		}
		out = append(out, kyvernov1.ResourceFilter{
			ResourceDescription: kyvernov1.ResourceDescription{Kinds: expandExcludedKind(r.Kind), Names: []string{r.Name}},
		})
	}
	return out
}

// The scale rules match `K/scale` and Kyverno plain kinds never match a subresource, so an
// excluded K without its scale subresource would leave `kubectl scale` of it denied.
func expandExcludedKind(kind string) []string {
	out := []string{kind}
	for _, s := range ScaleSubresourceKinds {
		if baseKind(s) == kind {
			out = append(out, s)
		}
	}
	return out
}

// ponytail: a template whose every kind is excluded still renders, with a total exclude; add a
// skip when a plan hits it in practice.
func applyExclusions(pol *kyvernov1.Policy, filters kyvernov1.ResourceFilters) {
	if len(filters) == constants.DefaultInitValue {
		return
	}
	for i := range pol.Spec.Rules {
		rule := &pol.Spec.Rules[i]
		if rule.ExcludeResources == nil {
			rule.ExcludeResources = &kyvernov1.MatchResources{}
		}
		rule.ExcludeResources.Any = append(rule.ExcludeResources.Any, filters.DeepCopy()...)
	}
}
