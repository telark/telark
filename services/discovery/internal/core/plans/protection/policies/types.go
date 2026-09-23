package policies

import "k8s.io/apimachinery/pkg/runtime/schema"

const (
	FieldManager        = "telark-protection-plans"
	labelSelectorFormat = "%s=%s"
	errDeletePolicyFmt  = "delete policy %s/%s: %w"
)

var KyvernoPolicyGVR = schema.GroupVersionResource{
	Group:    "kyverno.io",
	Version:  "v1",
	Resource: "policies",
}

type NamespacedName struct {
	Namespace string
	Name      string
}
