package config

import (
	"sync"
	"sync/atomic"

	"github.com/telark/discovery/internal/constants"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

var (
	snapshotsMaxOnce sync.Once
	snapshotsMaxVal  int
	snapshotsMaxLive atomic.Int64
)

// SetSnapshotsMaxVersions publishes the value from the global-config sync loop so
// a settings change applies to the next recorded change, not the next restart.
func SetSnapshotsMaxVersions(n int) {
	if n >= constants.MinSnapshotsMaxVersions {
		snapshotsMaxLive.Store(int64(n))
	}
}

func SnapshotsMaxVersions() int {
	if live := snapshotsMaxLive.Load(); live >= int64(constants.MinSnapshotsMaxVersions) {
		return int(live)
	}
	snapshotsMaxOnce.Do(initSnapshotsMaxVersions)
	return snapshotsMaxVal
}

func initSnapshotsMaxVersions() {
	cfg, err := gcfgclient.NewClient().GetGlobalConfig()
	if err != nil || cfg == nil {
		snapshotsMaxVal = constants.DefaultSnapshotsMaxVersions
		return
	}
	n := cfg.Snapshots.MaxPerApp
	if n < constants.MinSnapshotsMaxVersions {
		snapshotsMaxVal = constants.DefaultSnapshotsMaxVersions
		return
	}
	snapshotsMaxVal = n
}
