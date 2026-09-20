package policies

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers every renderer
)

// Params the two parameterised templates require; the rest render without any.
var templateParams = map[string]map[string]any{
	"block-image-types": {"imagePatterns": []string{"*nginx*"}},
	"block-image-tags":  {"tags": []string{"latest"}},
}

// Kyverno defaults allowExistingViolations to true, which would let an already-violating
// workload keep being modified for the whole protection window.
func TestEveryRenderedRuleForbidsExistingViolations(t *testing.T) {
	for _, tpl := range plans.Templates {
		t.Run(tpl.ID, func(t *testing.T) {
			plan := &plans.ProtectionPlan{
				ID:       "pp-abc-1234-5678",
				Name:     "freeze",
				Mode:     plans.ModeEnforce,
				Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}},
				Policies: []plans.ProtectionPlanPolicy{{TemplateID: tpl.ID, Params: templateParams[tpl.ID]}},
			}
			rendered, err := policies.Render(plan, nil, nil)
			if err != nil {
				t.Fatalf("render %s: %v", tpl.ID, err)
			}
			if len(rendered) == constants.DefaultInitValue {
				t.Fatalf("template %s rendered no policy", tpl.ID)
			}
			for i := range rendered {
				assertRulesForbidExistingViolations(t, tpl.ID, rendered[i].Spec.Rules)
			}
		})
	}
}

func assertRulesForbidExistingViolations(t *testing.T, templateID string, rules []kyvernov1.Rule) {
	t.Helper()
	if len(rules) == constants.DefaultInitValue {
		t.Fatalf("template %s rendered a policy with no rules", templateID)
	}
	for _, rule := range rules {
		if rule.Validation == nil {
			t.Fatalf("template %s rule %s has no validation block", templateID, rule.Name)
		}
		if rule.HasValidateAllowExistingViolations() {
			t.Errorf("template %s rule %s allows pre-existing violations", templateID, rule.Name)
		}
	}
}
