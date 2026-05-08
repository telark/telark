package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockCreate = "block-create"

type blockCreate struct{}

func (blockCreate) TemplateID() string { return templateBlockCreate }

func (blockCreate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockCreate, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:           templateBlockCreate,
			MatchResources: policies.MatchAllAny(kindsWildcard, opsCreate, scope.ApplicationIDs),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockCreate, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opEquals, opCreate),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockCreate{})
}
