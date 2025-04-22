package common

import (
	v1 "k8s.io/api/admissionregistration/v1"
)

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

type ServiceConfig struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Path      string `json:"path"`
	Port      int32  `json:"port"`
}

var ALL_OPERATIONS_EXCEPT_CONNECT = []v1.OperationType{
	v1.Create,
	v1.Update,
	v1.Delete,
}

type EnforceType string

const (
	IGNORED  EnforceType = "Ignore"
	ENFORCED EnforceType = "Fail"
)

type WebhookType string

const (
	VALIDATING WebhookType = "validating"
	MUTATING   WebhookType = "mutating"
)
