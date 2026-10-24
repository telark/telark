package policies

import (
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	FieldManager        = "telark-protection-plans"
	labelSelectorFormat = "%s=%s"
	errDeletePolicyFmt  = "delete policy %s/%s: %w"
	errReplacePolicyFmt = "replace policy %s/%s: %w"
	resourceNVPolicies  = "namespacedvalidatingpolicies"
	fieldSpec           = "spec"
	fieldActions        = "validationActions"
)

var KyvernoPolicyGVR = schema.GroupVersionResource{
	Group:    policiesv1.GroupVersion.Group,
	Version:  policiesv1.GroupVersion.Version,
	Resource: resourceNVPolicies,
}

type NamespacedName struct {
	Namespace string
	Name      string
}
