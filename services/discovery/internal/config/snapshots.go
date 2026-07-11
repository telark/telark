package config

import (
	"sync"

	"github.com/telark/discovery/internal/constants"
	gcfgclient "github.com/telark/rest/clients/resources/globalconfig"
)

var (
	snapshotsMaxOnce sync.Once
	snapshotsMaxVal  int
)

func SnapshotsMaxVersions() int {
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
