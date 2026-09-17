package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/internal/constants"
)

type AutoCleanupConfig struct {
	Enabled             bool
	DeleteEnabled       bool
	CycleInterval       time.Duration
	EmptyCyclesRequired int
	GracePeriod         time.Duration
}

func LoadAutoCleanupConfig() AutoCleanupConfig {
	return AutoCleanupConfig{
		Enabled: envBool(
			constants.EnvAutoCleanupEnabled,
			constants.DefaultAutoCleanupEnabled,
		),
		DeleteEnabled: envBool(
			constants.EnvAutoCleanupDeleteEnabled,
			constants.DefaultAutoCleanupDeleteEnabled,
		),
		CycleInterval: envDurationSec(
			constants.EnvAutoCleanupCycleIntervalSec,
			constants.DefaultAutoCleanupCycleInterval,
		),
		EmptyCyclesRequired: envInt(
			constants.EnvAutoCleanupEmptyCyclesRequired,
			constants.DefaultAutoCleanupEmptyCyclesRequired,
		),
		GracePeriod: envGraceSec(
			constants.EnvAutoCleanupGracePeriodSec,
			constants.DefaultAutoCleanupGracePeriod,
		),
	}
}

// Unlike envDurationSec, zero is a valid grace period: delete on the first eligible cycle.
func envGraceSec(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < constants.DefaultInitValue {
		return fallback
	}
	return time.Duration(n) * time.Second
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == constants.EmptyString {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}
