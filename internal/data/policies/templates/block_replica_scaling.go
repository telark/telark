package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockReplicaScaling = "block-replica-scaling"
	codeBlockReplicaScaling     = "brs"
)

type blockReplicaScaling struct{}

func (blockReplicaScaling) TemplateID() string   { return templateBlockReplicaScaling }
func (blockReplicaScaling) TemplateCode() string { return codeBlockReplicaScaling }

func (blockReplicaScaling) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprNewReplicas, opNotEquals, exprOldReplicas),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockReplicaScaling,
		TemplateCode: codeBlockReplicaScaling,
		RuleName:     templateBlockReplicaScaling,
		Kinds:        kindsReplicaTarget,
		Ops:          opsUpdate,
		Message:      fmt.Sprintf(msgBlockReplicaScaling, meta.PlanName),
		Deny:         deny,
	}), nil
}

func init() {
	policies.Register(blockReplicaScaling{})
}
