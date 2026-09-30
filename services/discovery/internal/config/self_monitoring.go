package config

import "github.com/telark/telark/services/discovery/internal/constants"

func SelfMonitoringEnabled() bool {
	return envBool(constants.EnvSelfMonitoringEnabled, constants.DefaultSelfMonitoringEnabled)
}
