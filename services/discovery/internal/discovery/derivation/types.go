package derivation

import "time"

// OwnerReference is a minimal K8s owner reference for resolving parent groups
type OwnerReference struct {
	Kind string
	Name string
	UID  string
}

type ResourceInput struct {
	Namespace string
	Kind      string
	Name      string
	Labels    map[string]string
	CreatedAt time.Time
	// Populated during `services/application/core.EnrichInputsWithK8s(...)` by reading
	// the live Kubernetes object's metadata annotations (Kyverno-injected).
	LastModifiedBy  string
	LastModifiedAt  time.Time
	LastModifiedOp  string
	Images          []string
	Ports           []int
	EnvVarKeys      []string
	ConfigMapRefs   []string
	SecretRefs      []string
	ServiceMappings []string
	IngressRules    []string
	OwnerReferences []OwnerReference
}

type ResourceWithGroup struct {
	Namespace       string            `json:"namespace"`
	Kind            string            `json:"kind"`
	Name            string            `json:"name"`
	Labels          map[string]string `json:"labels,omitempty"`
	Group           string            `json:"group"`
	CreatedAt       time.Time
	LastModifiedBy  string
	LastModifiedAt  time.Time
	LastModifiedOp  string
	Images          []string
	Ports           []int
	EnvVarKeys      []string
	ConfigMapRefs   []string
	SecretRefs      []string
	ServiceMappings []string
	IngressRules    []string
}

type groupKey struct {
	namespace string
	appKey    string
}

type identityResult struct {
	appKey     string
	signal     string
	identified bool
}
