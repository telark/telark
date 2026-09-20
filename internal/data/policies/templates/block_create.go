package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	templateBlockCreate = "block-create"
	codeBlockCreate     = "bc"
)

type blockCreate struct{}

func (blockCreate) TemplateID() string   { return templateBlockCreate }
func (blockCreate) TemplateCode() string { return codeBlockCreate }

func (blockCreate) Render(meta policies.RenderMeta, scope policies.ScopeSpec, _ map[string]any) (*kyvernov1.Policy, error) {
	deny := policies.DenyWithConditions([]kyvernov1.Condition{
		policies.MakeCondition(exprRequestOperation, opEquals, opCreate),
	})
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockCreate,
		TemplateCode: codeBlockCreate,
		RuleName:     templateBlockCreate,
		Kinds:        kindsWildcard,
		Ops:          opsCreate,
		Message:      fmt.Sprintf(msgBlockCreate, meta.PlanName),
		Deny:         deny,
		// Matching only the application's current resource names blocks re-creation of a
		// deleted member but never a new resource joining the application.
		MatchAppIdentity: true,
		// Reaches the Pods and PVCs controllers create themselves, so a replacement pod must
		// still be admitted while the plan is active.
		ExcludeControllers: true,
	}), nil
}

func init() {
	policies.Register(blockCreate{})
}
