package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/plsyro/data/policies"
)

const (
	templateBlockUpdate = "block-update"
	codeBlockUpdate     = "bu"
)

type blockUpdate struct{}

func (blockUpdate) TemplateID() string   { return templateBlockUpdate }
func (blockUpdate) TemplateCode() string { return codeBlockUpdate }

func (blockUpdate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprRequestOperation, opEquals, opUpdate),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockUpdate,
		TemplateCode: codeBlockUpdate,
		RuleName:     templateBlockUpdate,
		Kinds:        kindsWildcard,
		Ops:          opsUpdate,
		Message:      fmt.Sprintf(msgBlockUpdate, meta.PlanName),
		Deny:         deny,
	}), nil
}

func init() {
	policies.Register(blockUpdate{})
}
