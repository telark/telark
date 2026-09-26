package telarkconfig

import (
	"context"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/startup"
	cfgclient "github.com/telark/rest/clients/config"
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
