package config

import (
	"testing"

	"github.com/telark/discovery/internal/config"
)

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
