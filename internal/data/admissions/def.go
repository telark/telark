package admissions

import v1 "k8s.io/api/admissionregistration/v1"

type WebhookConfig struct {
	Name          string        `json:"name"`
	FailurePolicy EnforceType   `json:"FailurePolicy"`
	URL           string        `json:"url"`
	Rules         []WebhookRule `json:"rules"`
}

type WebhookRule struct {
	Operations  []v1.OperationType `json:"pperations"`
	APIGroups   []string           `json:"apiGroups"`
	APIVersions []string           `json:"apiVersions"`
	Resources   []string           `json:"resources"`
}

type EnforceType string

const (
	IGNORED  EnforceType = "Ignore"
	ENFORCED EnforceType = "Fail"
)
