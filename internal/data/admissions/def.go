package admissions

import v1 "k8s.io/api/admissionregistration/v1"

type WebhookConfig struct {
	Name          string       `json:"name"`
	URL           string       `json:"url"`
	Rules         []Rule       `json:"rules"`
	Client        ClientConfig `json:"client"`
	FailurePolicy EnforceType  `json:"FailurePolicy"`
}

type Rule struct {
	Operations  []v1.OperationType `json:"pperations"`
	APIGroups   []string           `json:"apiGroups"`
	APIVersions []string           `json:"apiVersions"`
	Resources   []string           `json:"resources"`
	Scope       string             `json:"scope"`
}

type ClientConfig struct {
	Service  ServiceConfig `json:"service"`
	CaBundle string        `json:"caBundle"`
}

type ServiceConfig struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	Port      int32  `json:"port"`
}

type EnforceType string

const (
	IGNORED  EnforceType = "Ignore"
	ENFORCED EnforceType = "Fail"
)
