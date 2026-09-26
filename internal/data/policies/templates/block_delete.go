package templates

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockDelete = "block-delete"
	codeBlockDelete     = "bd"
)

type blockDelete struct{}

func (blockDelete) TemplateID() string   { return templateBlockDelete }
func (blockDelete) TemplateCode() string { return codeBlockDelete }

func (blockDelete) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprRequestOperation, opEquals, opDelete),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockDelete,
		TemplateCode: codeBlockDelete,
		RuleName:     templateBlockDelete,
		Kinds:        kindsWildcard,
		Ops:          opsDelete,
		Message:      blockMessage(meta, msgBlockDelete),
		Deny:         deny,
		// Reaches the Pods, ReplicaSets and PVCs controllers create and delete themselves.
		ExcludeControllers: true,
	}), nil
}

func init() {
	policies.Register(blockDelete{})
}
