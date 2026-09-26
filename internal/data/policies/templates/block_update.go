package templates

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockUpdate  = "block-update"
	codeBlockUpdate      = "bu"
	ruleBlockUpdateScale = templateBlockUpdate + "-scale"
)

type blockUpdate struct{}

func (blockUpdate) TemplateID() string   { return templateBlockUpdate }
func (blockUpdate) TemplateCode() string { return codeBlockUpdate }

func (blockUpdate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprRequestOperation, opEquals, opUpdate),
	})
	message := blockMessage(meta, msgBlockUpdate)
	pol := policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockUpdate,
		TemplateCode: codeBlockUpdate,
		RuleName:     templateBlockUpdate,
		Kinds:        kindsWildcard,
		Ops:          opsUpdate,
		Message:      message,
		Deny:         deny,
		// The wildcard reaches the PVC a controller is binding and the ReplicaSet a rollout is
		// resizing, so those writes have to stay exempt.
		ExcludeControllers: true,
	})
	if pol == nil {
		return nil, nil
	}
	// A separate rule because Kyverno rejects `*` mixed with another kind. It must NOT exempt the
	// controller identities: the HPA controller shares them and `kubectl autoscale` went through.
	if match, ok := policies.BuildMatch(scope, policies.ScaleSubresourceKinds, opsUpdate); ok {
		pol.Spec.Rules = append(pol.Spec.Rules, kyvernov1.Rule{
			Name:             ruleBlockUpdateScale,
			MatchResources:   match,
			ExcludeResources: policies.ExcludePlatformWrites(),
			Validation:       policies.Validation(message, deny),
		})
	}
	return pol, nil
}

func init() {
	policies.Register(blockUpdate{})
}
