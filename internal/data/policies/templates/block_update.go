package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockUpdate = "block-update"
	codeBlockUpdate     = "bu"
)

type blockUpdate struct{}

func (blockUpdate) TemplateID() string   { return templateBlockUpdate }
func (blockUpdate) TemplateCode() string { return codeBlockUpdate }
func (blockUpdate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	match, ok := policies.BuildMatch(scope, kindsWildcard, opsUpdate)
	if !ok {
		return nil, nil
	}
	pol := policies.PolicyShell(meta, templateBlockUpdate, codeBlockUpdate, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockUpdate,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
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
