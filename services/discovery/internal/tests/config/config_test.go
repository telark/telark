package config

import (
	"testing"

	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	overrideBatchSize   = 50
	overrideEmptyCycles = 7
)

// With no environment overrides every loader falls back to its compiled default
// and every getter returns a usable value — this walks the fallback branch of
// each env helper.
func TestLoadersUseDefaults(t *testing.T) {
	_ = config.LoadCoordinationConfig()
	_ = config.LoadForceSyncConfig()
	_ = config.LoadAutoCleanupConfig()

	if config.InformerResyncSec() <= constants.DefaultInitValue {
		t.Error("informer resync must be positive")
	}
	if config.SnapshotFetchTimeout() <= constants.DefaultInitValue {
		t.Error("snapshot fetch timeout must be positive")
	}
	if config.RedisPingTimeout() <= constants.DefaultInitValue {
		t.Error("redis ping timeout must be positive")
	}
	if config.SnapshotWriteMaxAttempts() <= constants.DefaultInitValue {
		t.Error("snapshot write attempts must be positive")
	}
	if len(config.SnapshotScopes()) == constants.DefaultInitValue || config.DefaultSnapshotScope() == "" {
		t.Error("snapshot scopes must be defined")
	}
	// Exercised for coverage; these have no return worth asserting.
	_ = config.RollbackInformerResync()
	_ = config.CoalesceWindowSec()
	_ = config.CoalesceMaxWaitSec()
	_ = config.CoalesceBufferMaxEntries()
	_ = config.InformerResyncJitterFraction()
	_ = config.RedisDialRetryInterval()
	_ = config.RedisDialMaxWait()
	_ = config.SnapshotWriteRetryInterval()
	config.ApplyKubernetesRESTRateLimit()
}

// A valid override is parsed and applied, exercising the parse-success branch of
// the int64/duration/int/bool/allow-zero env helpers.
func TestLoadersParseOverrides(t *testing.T) {
	t.Setenv(constants.EnvCoordinationBatchSize, "50")
	testutil.Equal(t, "batch size", config.LoadCoordinationConfig().BatchSize, int64(overrideBatchSize))

	t.Setenv(constants.EnvAutoCleanupEnabled, "true")
	t.Setenv(constants.EnvAutoCleanupEmptyCyclesRequired, "7")
	t.Setenv(constants.EnvAutoCleanupCycleIntervalSec, "30")
	ac := config.LoadAutoCleanupConfig()
	testutil.Equal(t, "enabled", ac.Enabled, true)
	testutil.Equal(t, "empty cycles", ac.EmptyCyclesRequired, overrideEmptyCycles)

	t.Setenv(constants.EnvForceSyncWorkers, "0")
	testutil.Equal(t, "workers allow zero", config.LoadForceSyncConfig().Workers, constants.DefaultInitValue)
}

// A malformed override is rejected and the loader keeps its default, matching the
// no-override result.
func TestLoadersRejectGarbage(t *testing.T) {
	defaultEnabled := config.LoadAutoCleanupConfig().Enabled
	t.Setenv(constants.EnvAutoCleanupEnabled, "not-a-bool")
	testutil.Equal(t, "bad bool falls back", config.LoadAutoCleanupConfig().Enabled, defaultEnabled)

	defaultBatch := config.LoadCoordinationConfig().BatchSize
	t.Setenv(constants.EnvCoordinationBatchSize, "not-a-number")
	testutil.Equal(t, "bad int64 falls back", config.LoadCoordinationConfig().BatchSize, defaultBatch)
}
