package policies

import kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"

type ApplicationResourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

type ResolvedApp struct {
	Namespace string
	Resources []ApplicationResourceRef
}

type ScopeSpec struct {
	Namespace      string
	ApplicationIDs []string
	AppResources   []ApplicationResourceRef
}

type TemplateRenderer interface {
	TemplateID() string
	TemplateCode() string
	Render(meta RenderMeta, scope ScopeSpec, params map[string]any) (*kyvernov1.Policy, error)
}

type RenderMeta struct {
	PlanID    string
	PlanName  string
	CreatedBy string
	Mode      string
}

type Logger interface {
	Info(msg string)
	Error(msg string)
}
