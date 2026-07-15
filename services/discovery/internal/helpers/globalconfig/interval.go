package globalconfig

import (
	"context"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/startup"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

func FetchIntervalSeconds(ctx context.Context) int {
	if !startup.WaitForGlobalConfigReady(ctx) {
		return constants.DefaultInitValue
	}
	cfg, err := gcfgclient.NewClient().GetGlobalConfig()
	if err != nil || cfg == nil {
		return constants.DefaultInitValue
	}
	return cfg.UserSettings.FetchIntervalSeconds
}
