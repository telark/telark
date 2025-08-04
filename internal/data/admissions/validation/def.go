package admissions

import (
	admissionsShared "github.com/plsyro/data-pkg/admissions/shared"
	globalShared "github.com/plsyro/data-pkg/shared"
)

type ValidationWebhookConfig struct {
	Name          string                        `json:"name"`
	Rules         []admissionsShared.Rule       `json:"rules"`
	Client        admissionsShared.ClientConfig `json:"client"`
	Update        globalShared.Action           `json:"update"`
	Delete        globalShared.Action           `json:"delete"`
	FailurePolicy admissionsShared.EnforceType  `json:"FailurePolicy"`
}
