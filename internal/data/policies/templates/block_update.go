package templates

import (
	"fmt"

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
	message := fmt.Sprintf(msgBlockUpdate, meta.PlanName)
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
	// A bare `*` never matches a subresource, so `kubectl scale` walked past a rule that claims
	// to block every update. It cannot join the rule above: Kyverno rejects a match entry that
	// mixes `*` with any other kind.
	//
	// This rule deliberately does NOT exempt the controller identities: the scale subresource is
	// user-authored spec, and the HPA controller shares those identities, so exempting them here
	// let `kubectl autoscale` scale a workload straight through the freeze.
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
