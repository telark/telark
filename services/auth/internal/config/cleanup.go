package config

import (
	"time"

	"github.com/telark/auth/internal/constants"
)

type CleanupConfig struct {
	ReconcileTick         time.Duration
	ReconcilePassDeadline time.Duration
	WorkersPerType        int
	StreamMaxLen          int64
	LagAlertThreshold     int64
	SweeperInterval       time.Duration
	JobMaxAttempts        int
	DedupTTL              time.Duration
	XClaimMinIdle         time.Duration
	ListTimeout           time.Duration
	PatchTimeout          time.Duration
	MaxConcurrentPatches  int
	BackoffInitial        time.Duration
	BackoffMax            time.Duration
}

func LoadCleanupConfig() CleanupConfig {
	return CleanupConfig{
		ReconcileTick: envSeconds(constants.EnvReconcileTickSeconds, constants.DefaultReconcileTickSeconds),
		ReconcilePassDeadline: envSeconds(constants.EnvReconcilePassDeadlineSeconds,
			constants.DefaultReconcilePassDeadlineSeconds),
		WorkersPerType: envInt(constants.EnvCleanupWorkersPerType, constants.DefaultCleanupWorkersPerType),
		StreamMaxLen: int64(envInt(constants.EnvCleanupStreamMaxLen,
			constants.DefaultCleanupStreamMaxLen)),
		LagAlertThreshold: int64(envInt(constants.EnvCleanupLagAlertThreshold,
			constants.DefaultCleanupLagAlertThreshold)),
		SweeperInterval: envSeconds(constants.EnvCleanupSweeperIntervalSeconds,
			constants.DefaultCleanupSweeperIntervalSeconds),
		JobMaxAttempts: envInt(constants.EnvCleanupJobMaxAttempts,
			constants.DefaultCleanupJobMaxAttempts),
		DedupTTL: envSeconds(constants.EnvCleanupDedupTTLSeconds,
			constants.DefaultCleanupDedupTTLSeconds),
		XClaimMinIdle: envSeconds(constants.EnvCleanupXClaimMinIdleSeconds,
			constants.DefaultCleanupXClaimMinIdleSeconds),
		ListTimeout: envSeconds(constants.EnvCleanupListTimeoutSeconds,
			constants.DefaultCleanupListTimeoutSeconds),
		PatchTimeout: envSeconds(constants.EnvCleanupPatchTimeoutSeconds,
			constants.DefaultCleanupPatchTimeoutSeconds),
		MaxConcurrentPatches: envInt(constants.EnvCleanupMaxConcurrentPatches,
			constants.DefaultCleanupMaxConcurrentPatches),
		BackoffInitial: envSeconds(constants.EnvCleanupBackoffInitialSeconds,
			constants.DefaultCleanupBackoffInitialSeconds),
		BackoffMax: envSeconds(constants.EnvCleanupBackoffMaxSeconds,
			constants.DefaultCleanupBackoffMaxSeconds),
	}
}

type BackfillConfig struct {
	Enabled    bool
	BatchSize  int
	BatchPause time.Duration
}

func LoadBackfillConfig() BackfillConfig {
	return BackfillConfig{
		Enabled:   getEnvAsBool(constants.EnvBackfillFinalizersEnabled, false),
		BatchSize: envInt(constants.EnvBackfillBatchSize, constants.DefaultBackfillBatchSize),
		BatchPause: time.Duration(envInt(constants.EnvBackfillBatchPauseMS,
			constants.DefaultBackfillBatchPauseMS)) * time.Millisecond,
	}
}

func envInt(key string, def int) int {
	raw, _ := getEnvAsInt64OrDefault(key, int64(def))
	return int(raw)
}

func envSeconds(key string, defSeconds int) time.Duration {
	return time.Duration(envInt(key, defSeconds)) * time.Second
}
