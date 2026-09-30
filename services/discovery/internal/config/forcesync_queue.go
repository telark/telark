package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telark/telark/services/discovery/internal/constants"
)

type ForceSyncConfig struct {
	StreamKey           string
	ConsumerGroup       string
	Workers             int
	StreamMaxLen        int64
	DedupTTL            time.Duration
	JobTimeout          time.Duration
	MaintenanceInterval time.Duration
	PELIdleReclaim      time.Duration
	AckRetention        time.Duration
}

func LoadForceSyncConfig() ForceSyncConfig {
	return ForceSyncConfig{
		StreamKey:     constants.ForceSyncStreamKey,
		ConsumerGroup: constants.ForceSyncConsumerGroup,
		Workers: envIntAllowZero(
			constants.EnvForceSyncWorkers, constants.DefaultForceSyncWorkers),
		StreamMaxLen: envInt64(
			constants.EnvForceSyncStreamMaxLen, constants.DefaultForceSyncStreamMaxLen),
		DedupTTL: envDurationSec(
			constants.EnvForceSyncDedupTTLSec, constants.DefaultForceSyncDedupTTL),
		JobTimeout: envDurationSec(
			constants.EnvForceSyncJobTimeoutSec, constants.DefaultForceSyncJobTimeout),
		MaintenanceInterval: envDurationSec(
			constants.EnvForceSyncMaintenanceIntervalSec, constants.DefaultForceSyncMaintenanceInterval),
		PELIdleReclaim: envDurationSec(
			constants.EnvForceSyncPELIdleReclaimSec, constants.DefaultForceSyncPELIdleReclaim),
		AckRetention: envDurationSec(
			constants.EnvForceSyncAckRetentionSec, constants.DefaultForceSyncAckRetention),
	}
}

func envIntAllowZero(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < constants.DefaultInitValue {
		return fallback
	}
	return n
}
