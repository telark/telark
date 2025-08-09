package validation

import (
	admissionshared "github.com/plsyro/data/admissions/shared"
	globalshared "github.com/plsyro/data/shared"
)

type ValidationWebhookConfig struct {
	Name          string                       `json:"name"`
	Rules         []admissionshared.Rule       `json:"rules"`
	Client        admissionshared.ClientConfig `json:"client"`
	Update        globalshared.Action          `json:"update"`
	Delete        globalshared.Action          `json:"delete"`
	FailurePolicy admissionshared.EnforceType  `json:"FailurePolicy"`
}
