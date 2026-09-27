package startup

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/telark/discovery/internal/constants"
	cfgclient "github.com/telark/rest/clients/config"
)

var telarkConfigReady atomic.Bool

func EnsureTelarkConfigReadyAsync(ctx context.Context) {
	go waitUntilTelarkConfigReady(ctx)
}

func IsTelarkConfigReady() bool {
	return telarkConfigReady.Load()
}

func WaitForTelarkConfigReady(ctx context.Context) bool {
	if telarkConfigReady.Load() {
		return true
	}
	waitUntilTelarkConfigReady(ctx)
	return telarkConfigReady.Load()
}

func waitUntilTelarkConfigReady(ctx context.Context) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	loggedUnreachable := false
	for {
		if ctx.Err() != nil {
			return
		}
		cfg, err := cfgclient.NewClient().GetConfig()
		if err == nil && cfg != nil {
			if !telarkConfigReady.Load() {
				lg.Info(string(constants.InfoTelarkConfigAvailable))
			}
			telarkConfigReady.Store(true)
			return
		}
		if !loggedUnreachable {
			lg.Warn(fmt.Sprintf(string(constants.WarnTelarkConfigUnavailable),
				int(constants.TelarkConfigReadyRetryBackoff/time.Second), err))
			loggedUnreachable = true
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(constants.TelarkConfigReadyRetryBackoff):
		}
	}
}
