package shared

import v1 "k8s.io/api/admissionregistration/v1"

type Rule struct {
	Operations  []v1.OperationType `json:"operations"`
	APIGroups   []string           `json:"apiGroups"`
	APIVersions []string           `json:"apiVersions"`
	Resources   []string           `json:"resources"`
	Scope       string             `json:"scope"`
}

type ClientConfig struct {
	Service  ServiceConfig `json:"service"`
	CaBundle string        `json:"caBundle"`
}

var AllOperationsExceptConnect = []v1.OperationType{
	v1.Create,
	v1.Update,
	v1.Delete,
}

type ServiceConfig struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	Port      int32  `json:"port"`
}

type EnforceType string

const (
	Ignored  EnforceType = "Ignore"
	Enforced EnforceType = "Fail"
)

type WebhookType string

const (
	Validating WebhookType = "validating"
	Mutating   WebhookType = "mutating"
)
