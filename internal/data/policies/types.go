package policies

import kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"

type ApplicationResourceRef struct {
	Kind      string
	Name      string
	Namespace string
}

type ResolvedApp struct {
	// Every namespace the application spans; each one gets its own rendered policy.
	Namespaces []string
	Resources  []ApplicationResourceRef
	// A PVC is referenced by an application, not owned, so it never appears in Resources; kept
	// apart so storage rules can reach claims without pulling PVCs into every other template.
	VolumeClaims []string
}

type ScopeSpec struct {
	Namespace       string
	ApplicationRefs []string
	AppResources    []ApplicationResourceRef
	VolumeClaims    []string
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
