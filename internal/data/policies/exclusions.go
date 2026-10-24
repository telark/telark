package policies

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// Resources outside the namespace are dropped: a namespaced policy cannot reach them anyway.
func ExclusionFilters(excl *plans.ProtectionPlanScopeExclusions, namespace string) []ResourceFilter {
	norm := plans.NormalizeExclusions(excl)
	if norm == nil {
		return nil
	}
	out := make([]ResourceFilter, constants.DefaultInitValue, len(norm.Kinds)+len(norm.Resources))
	for _, kind := range norm.Kinds {
		out = append(out, ResourceFilter{Kinds: expandExcludedKind(kind)})
	}
	for _, r := range norm.Resources {
		if r.Namespace != namespace {
			continue
		}
		out = append(out, ResourceFilter{Kinds: expandExcludedKind(r.Kind), Names: []string{r.Name}})
	}
	return out
}

// The scale branches match `K/scale` and a plain kind never matches a subresource, so an
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
func applyExclusions(pol *policiesv1.NamespacedValidatingPolicy, filters []ResourceFilter) {
	if len(filters) == constants.DefaultInitValue {
		return
	}
	pol.Spec.MatchConditions = append(pol.Spec.MatchConditions, admissionregistrationv1.MatchCondition{
		Name:       ConditionNotExcluded,
		Expression: celNegate(AnyOf(filters)),
	})
}
