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
	match, ok := policies.BuildMatch(scope, kindsReplicaTarget, opsUpdate)
	if !ok {
		return nil, nil
	}
	pol := policies.PolicyShell(meta, templateBlockReplicaScaling, codeBlockReplicaScaling, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockReplicaScaling,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlsyroManaged(),
			Validation: &kyvernov1.Validation{
				Message: fmt.Sprintf(msgBlockReplicaScaling, meta.PlanName),
				Deny: policies.DenyWithConditions([]kyvernov1.Condition{
					policies.MakeCondition(exprNewReplicas, opNotEquals, exprOldReplicas),
				}),
			},
		},
	}
	return pol, nil
}

func init() {
	policies.Register(blockReplicaScaling{})
}
