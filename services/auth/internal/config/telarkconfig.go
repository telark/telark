package config

import (
	"fmt"
	"sync"
	"time"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	"golang.org/x/sync/singleflight"
)

var (
	telarkConfigMu     sync.Mutex
	telarkConfig       telarkconfigresource.TelarkConfig
	telarkConfigReadAt time.Time
	telarkConfigFlight singleflight.Group
	telarkConfigLg     = constants.GetLogger(constants.LoggerPrefixAuthService)
)

// The public login config and every self-registration read this, so the exporter is asked at most
// once per TTL, even while failing; a failed read keeps the last value, which starts as the zero config (all off).
func TelarkConfig() telarkconfigresource.TelarkConfig {
	if cfg, fresh := cachedTelarkConfig(); fresh {
		return cfg
	}
	_, _, _ = telarkConfigFlight.Do(constants.TelarkConfigFlightKey, func() (any, error) {
		refreshTelarkConfig()
		return nil, nil
	})
	cfg, _ := cachedTelarkConfig()
	return cfg
}

func IsSelfRegistrationEnabled() bool {
	return TelarkConfig().SelfRegistration.Enabled
}

func cachedTelarkConfig() (telarkconfigresource.TelarkConfig, bool) {
	telarkConfigMu.Lock()
	defer telarkConfigMu.Unlock()
	return telarkConfig, time.Since(telarkConfigReadAt) < constants.TelarkConfigCacheTTL
}

func refreshTelarkConfig() {
	fetched, err := clients.GetConfigClient().GetConfig()
	telarkConfigMu.Lock()
	defer telarkConfigMu.Unlock()
	telarkConfigReadAt = time.Now()
	if err != nil {
		telarkConfigLg.Warn(fmt.Sprintf(string(constants.WarnTelarkConfigReadFailed), err))
		return
	}
	telarkConfig = *fetched
}
