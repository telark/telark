package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

type CoordinationConfig struct {
	BatchSize              int64
	BatchBlockDuration     time.Duration
	MaxRetryAttempts       int
	LockTTL                time.Duration
	LockHeartbeatInterval  time.Duration
	ElectionTTL            time.Duration
	ElectionRenewInterval  time.Duration
	ElectionResignTimeout  time.Duration
	DedupTTL               time.Duration
	StaleClaimMinIdle      time.Duration
	StaleClaimInterval     time.Duration
	ShutdownCleanupTimeout time.Duration
}

func LoadCoordinationConfig() CoordinationConfig {
	batchCount := int64(xwareredis.StreamDefaultReadCount)
	return CoordinationConfig{
		BatchSize: envInt64(constants.EnvCoordinationBatchSize, batchCount),
		BatchBlockDuration: envDurationSec(
			constants.EnvCoordinationBatchBlockSec,
			xwareredis.StreamDefaultBlockDuration,
		),
		MaxRetryAttempts: envInt(constants.EnvCoordinationMaxRetryAttempts, constants.ThreeValue),
		LockTTL:          envDurationSec(constants.EnvCoordinationLockTTLSec, xwareredis.LockDefaultTTL),
		LockHeartbeatInterval: envDurationSec(
			constants.EnvCoordinationLockHeartbeatSec,
			xwareredis.LockHeartbeatInterval,
		),
		ElectionTTL: envDurationSec(constants.EnvCoordinationElectionTTLSec, xwareredis.ElectionDefaultTTL),
		ElectionRenewInterval: envDurationSec(
			constants.EnvCoordinationElectionRenewSec,
			xwareredis.ElectionRenewInterval,
		),
		DedupTTL: envDurationSec(constants.EnvCoordinationDedupTTLSec, xwareredis.DedupTTL),
		StaleClaimMinIdle: envDurationSec(
			constants.EnvCoordinationStaleClaimMinIdleSec,
			xwareredis.StreamStaleClaimMinIdle,
		),
		StaleClaimInterval: envDurationSec(
			constants.EnvCoordinationStaleClaimIntervalSec,
			xwareredis.StreamStaleClaimInterval,
		),
		ShutdownCleanupTimeout: envDurationSec(
			constants.EnvCoordinationShutdownCleanupTimeoutSec,
			time.Duration(constants.CoordinationDefaultShutdownCleanupTimeoutSec)*time.Second,
		),
		ElectionResignTimeout: time.Duration(constants.CoordinationElectionResignTimeoutSec) * time.Second,
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= constants.DefaultInitValue {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, constants.IntBitSize64)
	if err != nil || v < constants.DefaultInitValue {
		return fallback
	}
	return v
}

func envInt64(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.ParseInt(raw, constants.IntBase10, constants.IntBitSize64)
	if err != nil || n <= constants.ZeroInt64 {
		return fallback
	}
	return n
}

func envDurationSec(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= constants.DefaultInitValue {
		return fallback
	}
	return time.Duration(n) * time.Second
}
