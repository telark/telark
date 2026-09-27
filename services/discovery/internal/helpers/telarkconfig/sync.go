package telarkconfig

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/startup"
	cfgclient "github.com/telark/rest/clients/config"
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
