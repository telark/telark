package config

import (
	"fmt"

	"github.com/telark/telark/services/auth/internal/constants"
)

var rateLimitLg = constants.GetLogger(constants.LoggerPrefixAuthService)

func RateLimitPerClient() int64 {
	return positiveOrDefault(constants.EnvRateLimitPerClient, constants.DefaultRateLimitPerClient)
}

func RateLimitGlobal() int64 {
	return positiveOrDefault(constants.EnvRateLimitGlobal, constants.DefaultRateLimitGlobal)
}

// Zero is valid: nothing in front of the ui, whose nginx appends the address it saw itself.
func RateLimitTrustedHops() int {
	hops, err := getEnvAsInt64OrDefault(constants.EnvRateLimitTrustedHops, constants.DefaultRateLimitTrustedHops)
	if err != nil || hops < constants.DefaultInitValue {
		rateLimitLg.Warn(fmt.Sprintf(string(constants.LogEnvInvalidHops), constants.EnvRateLimitTrustedHops,
			constants.DefaultRateLimitTrustedHops))
		return constants.DefaultRateLimitTrustedHops
	}
	return int(hops)
}

// A zero limit would refuse every start, so only a positive value is taken.
func positiveOrDefault(key string, fallback int64) int64 {
	value, err := getEnvAsInt64OrDefault(key, fallback)
	if err != nil || value <= constants.DefaultInitValue {
		rateLimitLg.Warn(fmt.Sprintf(string(constants.LogEnvInvalid), key, fallback))
		return fallback
	}
	return value
}
