package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockConfigSecretChanges = "block-config-secret-resource-changes"
	codeBlockConfigSecretChanges     = "bcsr"
)

type blockConfigSecretChanges struct{}

func (blockConfigSecretChanges) TemplateID() string   { return templateBlockConfigSecretChanges }
func (blockConfigSecretChanges) TemplateCode() string { return codeBlockConfigSecretChanges }

func (blockConfigSecretChanges) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockConfigSecretChanges,
		TemplateCode: codeBlockConfigSecretChanges,
		Kinds:        kindsConfigSecret,
		Ops:          opsUpdateDelete,
		Message:      blockMessage(meta, msgBlockConfigSecret),
		Admit:        admitNever,
	}), nil
}

func init() {
	policies.Register(blockConfigSecretChanges{})
}
