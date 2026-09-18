package policies

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/plans"
	datapolicies "github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
)

type PolicyTargetCombo struct {
	Policy plans.ProtectionPlanPolicy
	Target string
}

func Combinations(plist []plans.ProtectionPlanPolicy, targets []string) []PolicyTargetCombo {
	out := make([]PolicyTargetCombo, constants.DefaultInitValue, len(plist)*len(targets))
	for _, p := range plist {
		for _, t := range targets {
			out = append(out, PolicyTargetCombo{Policy: p, Target: t})
		}
	}
	return out
}

func renderForCombinations(
	plan *plans.ProtectionPlan,
	policiesSubset []plans.ProtectionPlanPolicy,
	targets []string,
	resolved map[string]datapolicies.ResolvedApp,
	logger datapolicies.Logger,
) ([]kyvernov1.Policy, error) {
	if len(policiesSubset) == constants.DefaultInitValue || len(targets) == constants.DefaultInitValue {
		return nil, nil
	}
	subPlan := &plans.ProtectionPlan{
		ID:        plan.ID,
		Name:      plan.Name,
		CreatedBy: plan.CreatedBy,
		Mode:      plan.Mode,
		Scope:     plans.ProtectionPlanScope{Type: plan.Scope.Type},
		Policies:  policiesSubset,
	}
	if plan.Scope.Type == plans.ScopeTypeApplications {
		subPlan.Scope.ApplicationIDs = targets
	} else {
		subPlan.Scope.Namespaces = targets
	}
	return datapolicies.Render(subPlan, resolved, logger)
}

func renderCombos(
	plan *plans.ProtectionPlan,
	combos []PolicyTargetCombo,
	resolved map[string]datapolicies.ResolvedApp,
	logger datapolicies.Logger,
) ([]kyvernov1.Policy, error) {
	if len(combos) == constants.DefaultInitValue {
		return nil, nil
	}
	grouped := map[string][]plans.ProtectionPlanPolicy{}
	for _, c := range combos {
		grouped[c.Target] = append(grouped[c.Target], c.Policy)
	}
	out := make([]kyvernov1.Policy, constants.DefaultInitValue, len(combos))
	for target, plist := range grouped {
		rendered, err := renderForCombinations(plan, plist, []string{target}, resolved, logger)
		if err != nil {
			return nil, err
		}
		out = append(out, rendered...)
	}
	return out, nil
}

func renderedPolicyRefs(rendered []kyvernov1.Policy) []NamespacedName {
	refs := make([]NamespacedName, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		refs = append(refs, NamespacedName{
			Namespace: rendered[i].Namespace,
			Name:      rendered[i].Name,
		})
	}
	return refs
}

func computeRemovalNames(
	plan *plans.ProtectionPlan,
	combos []PolicyTargetCombo,
	resolved map[string]datapolicies.ResolvedApp,
) ([]string, error) {
	if len(combos) == constants.DefaultInitValue {
		return nil, nil
	}
	names := make([]string, constants.DefaultInitValue, len(combos))
	for _, c := range combos {
		scope, err := scopeForTarget(plan.Scope.Type, c.Target, resolved)
		if err != nil {
			return nil, err
		}
		r, ok := datapolicies.GetRenderer(c.Policy.TemplateID)
		if !ok {
			return nil, fmt.Errorf("policies: no renderer registered for template %q", c.Policy.TemplateID)
		}
		names = append(names, datapolicies.PolicyName(plan.ID, r.TemplateCode(), scope))
	}
	return names, nil
}

func scopeForTarget(
	scopeType, target string,
	resolved map[string]datapolicies.ResolvedApp,
) (datapolicies.ScopeSpec, error) {
	if scopeType == plans.ScopeTypeNamespaces {
		return datapolicies.ScopeSpec{Namespace: target}, nil
	}
	ra, ok := resolved[target]
	if !ok || ra.Namespace == constants.EmptyString {
		return datapolicies.ScopeSpec{}, fmt.Errorf("application %q has no resolved namespace", target)
	}
	return datapolicies.ScopeSpec{
		Namespace:      ra.Namespace,
		ApplicationIDs: []string{target},
		AppResources:   ra.Resources,
	}, nil
}
