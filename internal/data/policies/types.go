package policies

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

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
	Render(meta RenderMeta, scope ScopeSpec, params map[string]any) (*policiesv1.NamespacedValidatingPolicy, error)
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

// One branch of a match or of an exclusion: the CEL stage ANDs the set fields and ORs the branches.
type ResourceFilter struct {
	// A kind, "<Kind>/scale", or KindWildcard.
	Kinds []string
	// Exact names, or names with * and ? wildcards.
	Names  []string
	AppIDs []string
	// Only route requests to the policy; the CEL never re-checks them.
	Ops []string
	// Set only on branches that reach objects a controller writes itself; see ExceptControllers.
	ExceptControllers bool
}

type PolicySpec struct {
	TemplateID   string
	TemplateCode string
	Match        []ResourceFilter
	Variables    []admissionregistrationv1.Variable
	Validations  []admissionregistrationv1.Validation
}

type ruleGroup struct {
	group     string
	ops       []string
	resources []string
}
