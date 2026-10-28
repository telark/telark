package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockReplicaScaling = "block-replica-scaling"
	codeBlockReplicaScaling     = "brs"
)

type blockReplicaScaling struct{}

func (blockReplicaScaling) TemplateID() string   { return templateBlockReplicaScaling }
func (blockReplicaScaling) TemplateCode() string { return codeBlockReplicaScaling }

func (blockReplicaScaling) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockReplicaScaling,
		TemplateCode: codeBlockReplicaScaling,
		Kinds:        kindsReplicaTarget,
		Ops:          opsUpdate,
		Message:      blockMessage(meta, msgBlockReplicaScaling),
		Admit:        admitSameReplicas,
	}), nil
}

func init() {
	policies.Register(blockReplicaScaling{})
}
