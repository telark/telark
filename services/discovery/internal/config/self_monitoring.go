package config

import "github.com/telark/discovery/internal/constants"

func SelfMonitoringEnabled() bool {
	return envBool(constants.EnvSelfMonitoringEnabled, constants.DefaultSelfMonitoringEnabled)
}
