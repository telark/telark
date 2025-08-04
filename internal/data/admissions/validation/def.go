package admissions

import (
	"github.com/plsyro/data-pkg/admissions/common"
	globalShared "github.com/plsyro/data-pkg/shared"
)

type ValidationWebhookConfig struct {
	Name          string              `json:"name"`
	Rules         []common.Rule       `json:"rules"`
	Client        common.ClientConfig `json:"client"`
	Update        globalShared.Action `json:"update"`
	Delete        globalShared.Action `json:"delete"`
	FailurePolicy common.EnforceType  `json:"FailurePolicy"`
}
