package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockUpdate = "block-update"

type blockUpdate struct{}

func (blockUpdate) TemplateID() string { return templateBlockUpdate }

func (blockUpdate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockUpdate, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:           templateBlockUpdate,
			MatchResources: policies.MatchAllAny(kindsWildcard, opsUpdate, scope.ApplicationIDs),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockUpdate, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opEquals, opUpdate),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockUpdate{})
}
