package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockConfigSecretChanges = "block-config-secret-resource-changes"

type blockConfigSecretChanges struct{}

func (blockConfigSecretChanges) TemplateID() string { return templateBlockConfigSecretChanges }

func (blockConfigSecretChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockConfigSecretChanges, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockConfigSecretChanges,
			MatchResources:   policies.MatchAllAny(kindsConfigSecret, opsUpdateDelete, scope.ApplicationIDs),
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
