package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockConfigSecretChanges = "block-config-secret-resource-changes"
	codeBlockConfigSecretChanges     = "bcsr"
)

type blockConfigSecretChanges struct{}

func (blockConfigSecretChanges) TemplateID() string   { return templateBlockConfigSecretChanges }
func (blockConfigSecretChanges) TemplateCode() string { return codeBlockConfigSecretChanges }
func (blockConfigSecretChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	match, ok := policies.BuildMatch(scope, kindsConfigSecret, opsUpdateDelete)
	if !ok {
		return nil, nil
	}
	pol := policies.PolicyShell(meta, templateBlockConfigSecretChanges, codeBlockConfigSecretChanges, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockConfigSecretChanges,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockConfigSecret, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprRequestOperation, opIn, opsUpdateDelete),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockConfigSecretChanges{})
}
