package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const templateBlockReplicaScaling = "block-replica-scaling"

type blockReplicaScaling struct{}

func (blockReplicaScaling) TemplateID() string { return templateBlockReplicaScaling }

func (blockReplicaScaling) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	pol := policies.PolicyShell(meta, templateBlockReplicaScaling, scope)
	pol.Spec.Rules = []kyvernov1.Rule{
		{
			Name:             templateBlockReplicaScaling,
			MatchResources:   policies.MatchAllAny(kindsReplicaTarget, opsUpdate, scope.ApplicationIDs),
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
