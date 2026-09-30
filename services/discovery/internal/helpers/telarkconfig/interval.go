package telarkconfig

import (
	"context"

	cfgclient "github.com/telark/telark/internal/rest/clients/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/startup"
)

func FetchIntervalSeconds(ctx context.Context) int {
	if !startup.WaitForTelarkConfigReady(ctx) {
		return constants.DefaultInitValue
	}
	cfg, err := cfgclient.NewClient().GetConfig()
	if err != nil || cfg == nil {
		return constants.DefaultInitValue
	}
	return cfg.UserSettings.FetchIntervalSeconds
}
