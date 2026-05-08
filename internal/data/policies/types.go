package policies

import kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"

type ScopeSpec struct {
	Namespace      string
	ApplicationIDs []string
}

type TemplateRenderer interface {
	TemplateID() string
	Render(meta RenderMeta, scope ScopeSpec, params map[string]any) (*kyvernov1.Policy, error)
}

type RenderMeta struct {
	PlanID    string
	PlanName  string
	CreatedBy string
	Mode      string
}
