package config

import (
	"fmt"
	"testing"

	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	overrideRollbackWorkers = 8
	overrideQPS             = 12
	overrideBurst           = 24
)

// Anything unparsable or non-positive falls back to the default worker count.
func TestRollbackWorkers(t *testing.T) {
	cases := map[string]int{
		"":    constants.DefaultRollbackWorkers,
		"8":   overrideRollbackWorkers,
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
	if config.InformerResyncSec() <= constants.DefaultInitValue {
		t.Fatal("informer resync should be positive")
	}
	if config.RollbackInformerResync() <= constants.DefaultInitValue {
		t.Fatal("rollback resync should be positive")
	}
	if config.CoalesceWindowSec() <= constants.DefaultInitValue {
		t.Fatal("coalesce window should be positive")
	}
	_ = config.CoalesceMaxWaitSec()
	_ = config.CoalesceBufferMaxEntries()
	if config.InformerResyncJitterFraction() < constants.DefaultInitValue {
		t.Fatal("jitter fraction should be non-negative")
	}
	if config.SnapshotFetchTimeout() <= constants.DefaultInitValue {
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
	if len(config.SnapshotScopes()) == constants.DefaultInitValue {
		t.Fatal("snapshot scopes should be defined")
	}

	// Composite loaders read several env keys; ensure they assemble cleanly.
	_ = config.LoadCoordinationConfig()
	_ = config.LoadAutoCleanupConfig()
	_ = config.LoadForceSyncConfig()
}

func TestRollbackK8sClientRateLimit(t *testing.T) {
	cases := []struct {
		qps, burst string
		wantQPS    float32
		wantBurst  int
	}{
		{constants.EmptyString, constants.EmptyString, float32(constants.DefaultRollbackK8sClientQPS), constants.DefaultRollbackK8sClientBurst},
		{"12", "24", overrideQPS, overrideBurst},
		{"bad", "-1", float32(constants.DefaultRollbackK8sClientQPS), constants.DefaultRollbackK8sClientBurst},
	}
	for _, c := range cases {
		t.Setenv(constants.EnvDiscoveryRollbackK8sClientQPS, c.qps)
		t.Setenv(constants.EnvDiscoveryRollbackK8sClientBurst, c.burst)
		qps, burst := config.RollbackK8sClientRateLimit()
		testutil.Equal(t, fmt.Sprintf("qps for %q", c.qps), qps, c.wantQPS)
		testutil.Equal(t, fmt.Sprintf("burst for %q", c.burst), burst, c.wantBurst)
	}
}
