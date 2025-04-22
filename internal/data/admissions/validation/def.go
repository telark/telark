package admissions

import (
	"github.com/plsyro/data-pkg/admissions/common"
	baseCommon "github.com/plsyro/data-pkg/common"
)

type ValidationWebhookConfig struct {
	Name          string              `json:"name"`
	Rules         []common.Rule       `json:"rules"`
	Client        common.ClientConfig `json:"client"`
	Update        baseCommon.Action   `json:"update"`
	Delete        baseCommon.Action   `json:"delete"`
	FailurePolicy common.EnforceType  `json:"FailurePolicy"`
}
