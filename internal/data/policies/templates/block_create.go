package templates

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/policies"
)

const (
	templateBlockCreate = "block-create"
	codeBlockCreate     = "bc"
)

type blockCreate struct{}

func (blockCreate) TemplateID() string   { return templateBlockCreate }
func (blockCreate) TemplateCode() string { return codeBlockCreate }

func (blockCreate) Render(
	meta policies.RenderMeta,
	scope policies.ScopeSpec,
	_ map[string]any,
) (*policiesv1.NamespacedValidatingPolicy, error) {
	return policies.RenderSingleRulePolicy(meta, scope, policies.SingleRuleSpec{
		TemplateID:   templateBlockCreate,
		TemplateCode: codeBlockCreate,
		Kinds:        kindsWildcard,
		Ops:          opsCreate,
		Message:      blockMessage(meta, msgBlockCreate),
		Admit:        admitNever,
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
