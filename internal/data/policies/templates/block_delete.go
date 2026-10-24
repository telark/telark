package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockDelete = "block-delete"
	codeBlockDelete     = "bd"
)

type blockDelete struct{}

func (blockDelete) TemplateID() string   { return templateBlockDelete }
func (blockDelete) TemplateCode() string { return codeBlockDelete }

func (blockDelete) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockDelete,
		TemplateCode: codeBlockDelete,
		Kinds:        kindsWildcard,
		Ops:          opsDelete,
		Message:      blockMessage(meta, msgBlockDelete),
		Admit:        admitNever,
		// Reaches the Pods, ReplicaSets and PVCs controllers create and delete themselves.
		ExcludeControllers: true,
	}), nil
}

func init() {
	policies.Register(blockDelete{})
}
