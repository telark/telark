package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/telark/discovery/constants"
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
		GracePeriod: envDurationSec(
			constants.EnvAutoCleanupGracePeriodSec,
			constants.DefaultAutoCleanupGracePeriod,
		),
	}
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
