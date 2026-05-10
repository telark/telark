package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockCreate = "block-create"
	codeBlockCreate     = "bc"
)

type blockCreate struct{}

func (blockCreate) TemplateID() string   { return templateBlockCreate }
func (blockCreate) TemplateCode() string { return codeBlockCreate }
func (blockCreate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	match, ok := policies.BuildMatch(scope, kindsWildcard, opsCreate)
	if !ok {
		return nil, nil
	}
	pol := policies.PolicyShell(meta, templateBlockCreate, codeBlockCreate, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockCreate,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
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
