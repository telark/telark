package policies

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
)

const injectedPlanName = "{{request.object.data}}"

// Kyverno substitutes {{ }} in validate.message, so a user-chosen plan name there would leak
// admitted objects (Secret data) into violation events.
func TestPlanNameReachesOnlyTheAnnotation(t *testing.T) {
	for _, mode := range []string{plans.ModeEnforce, plans.ModeAudit} {
		for _, tpl := range plans.Templates {
			t.Run(mode+"/"+tpl.ID, func(t *testing.T) {
				plan := &plans.ProtectionPlan{
					ID:       multiPlanID,
					Name:     injectedPlanName,
					Mode:     mode,
					Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{planNamespace}},
					Policies: []plans.ProtectionPlanPolicy{{TemplateID: tpl.ID, Params: templateParams[tpl.ID]}},
				}
				rendered, err := policies.Render(plan, nil, nil)
				if err != nil {
					t.Fatalf(fmtRenderErr, err)
				}
				for i := range rendered {
					pol := rendered[i].DeepCopy()
					if pol.Annotations[policies.AnnotationPlanName] != injectedPlanName {
						t.Fatalf("plan-name annotation = %q", pol.Annotations[policies.AnnotationPlanName])
					}
					delete(pol.Annotations, policies.AnnotationPlanName)
					raw, merr := json.Marshal(pol)
					if merr != nil {
						t.Fatalf("marshal: %v", merr)
					}
					if strings.Contains(string(raw), injectedPlanName) {
						t.Errorf("plan name reached a substituted field: %s", raw)
					}
					for _, rule := range pol.Spec.Rules {
						if !strings.Contains(rule.Validation.Message, multiPlanID) {
							t.Errorf("rule %s message %q lacks the plan id", rule.Name, rule.Validation.Message)
						}
					}
				}
			})
		}
	}
}
