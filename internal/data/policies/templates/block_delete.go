package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockDelete = "block-delete"

type blockDelete struct{}

func (blockDelete) TemplateID() string { return templateBlockDelete }

func (blockDelete) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockDelete, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:           templateBlockDelete,
			MatchResources: policies.MatchAllAny(kindsWildcard, opsDelete, scope.ApplicationIDs),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockDelete, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opEquals, opDelete),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockDelete{})
}
