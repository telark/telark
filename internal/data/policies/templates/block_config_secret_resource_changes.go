package templates

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockConfigSecretChanges = "block-config-secret-resource-changes"
	codeBlockConfigSecretChanges     = "bcsr"
)

type blockConfigSecretChanges struct{}

func (blockConfigSecretChanges) TemplateID() string   { return templateBlockConfigSecretChanges }
func (blockConfigSecretChanges) TemplateCode() string { return codeBlockConfigSecretChanges }

func (blockConfigSecretChanges) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprRequestOperation, opIn, opsUpdateDelete),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockConfigSecretChanges,
		TemplateCode: codeBlockConfigSecretChanges,
		RuleName:     templateBlockConfigSecretChanges,
		Kinds:        kindsConfigSecret,
		Ops:          opsUpdateDelete,
		Message:      blockMessage(meta, msgBlockConfigSecret),
		Deny:         deny,
	}), nil
}

func init() {
	policies.Register(blockConfigSecretChanges{})
}
