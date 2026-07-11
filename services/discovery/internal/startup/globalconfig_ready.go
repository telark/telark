package startup

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/telark/discovery/internal/constants"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

var globalConfigReady atomic.Bool

func EnsureGlobalConfigReadyAsync(ctx context.Context) {
	go waitUntilGlobalConfigReady(ctx)
}

func WaitForGlobalConfigReady(ctx context.Context) bool {
	if globalConfigReady.Load() {
		return true
	}
	waitUntilGlobalConfigReady(ctx)
	return globalConfigReady.Load()
}

func waitUntilGlobalConfigReady(ctx context.Context) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	loggedUnreachable := false
	for {
		if ctx.Err() != nil {
			return
		}
		cfg, err := gcfgclient.NewClient().GetGlobalConfig()
		if err == nil && cfg != nil {
			if !globalConfigReady.Load() {
				lg.Info(string(constants.InfoGlobalConfigAvailable))
			}
			globalConfigReady.Store(true)
			return
		}
		if !loggedUnreachable {
			lg.Warn(fmt.Sprintf(string(constants.WarnGlobalConfigUnavailable),
				int(constants.GlobalConfigReadyRetryBackoff/time.Second), err))
			loggedUnreachable = true
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(constants.GlobalConfigReadyRetryBackoff):
		}
	}
}
