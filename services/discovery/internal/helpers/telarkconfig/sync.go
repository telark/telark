package telarkconfig

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	cfgclient "github.com/telark/telark/internal/rest/clients/config"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/startup"
)

var excludedCache atomic.Value

func StartExcludedNamespacesSync(ctx context.Context) {
	go runExcludedSync(ctx)
}

func runExcludedSync(ctx context.Context) {
	t := time.NewTicker(time.Duration(constants.TelarkConfigExcludedPollSec) * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			refreshExcluded(ctx)
		}
	}
}

func refreshExcluded(ctx context.Context) {
	if !startup.WaitForTelarkConfigReady(ctx) {
		return
	}
	cfg, err := cfgclient.NewClient().GetConfig()
	if err == nil && cfg == nil {
		err = errors.New("nil config")
	}
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnExcludedNamespacesRefresh), err),
		)
		return
	}
	excludedCache.Store(cfg.ExcludedNamespaces)
	config.SetSnapshotsMaxVersions(cfg.Snapshots.MaxPerApp)
}
