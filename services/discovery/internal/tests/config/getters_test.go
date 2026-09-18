package config

import (
	"fmt"
	"testing"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/tests/testutil"
)

// Anything unparsable or non-positive falls back to the default worker count.
func TestRollbackWorkers(t *testing.T) {
	cases := map[string]int{
		"":    constants.DefaultRollbackWorkers,
		"8":   8,
		"0":   constants.DefaultRollbackWorkers,
		"-3":  constants.DefaultRollbackWorkers,
		"abc": constants.DefaultRollbackWorkers,
	}
	for raw, want := range cases {
		t.Setenv(constants.EnvDiscoveryRollbackWorkers, raw)
		testutil.Equal(t, fmt.Sprintf("workers for %q", raw), config.RollbackWorkers(), want)
	}
}

// The exported configuration getters resolve without panicking and return usable
// values under the default (unset) environment, exercising their fallback paths.
func TestConfigGetters(t *testing.T) {
	if config.InformerResyncSec() <= 0 {
		t.Fatal("informer resync should be positive")
	}
	if config.RollbackInformerResync() <= 0 {
		t.Fatal("rollback resync should be positive")
	}
	if config.CoalesceWindowSec() <= 0 {
		t.Fatal("coalesce window should be positive")
	}
	_ = config.CoalesceMaxWaitSec()
	_ = config.CoalesceBufferMaxEntries()
	if config.InformerResyncJitterFraction() < 0 {
		t.Fatal("jitter fraction should be non-negative")
	}
	if config.SnapshotFetchTimeout() <= 0 {
		t.Fatal("snapshot fetch timeout should be positive")
	}
	_ = config.RedisDialRetryInterval()
	_ = config.RedisDialMaxWait()
	_ = config.RedisPingTimeout()
	_ = config.SnapshotWriteMaxAttempts()
	_ = config.SnapshotWriteRetryInterval()

	if config.DefaultSnapshotScope() == "" {
		t.Fatal("default snapshot scope should be non-empty")
	}
	if len(config.SnapshotScopes()) == 0 {
		t.Fatal("snapshot scopes should be defined")
	}

	// Composite loaders read several env keys; ensure they assemble cleanly.
	_ = config.LoadCoordinationConfig()
	_ = config.LoadAutoCleanupConfig()
	_ = config.LoadForceSyncConfig()
}
