package config

import (
	"fmt"
	"time"

	"github.com/telark/telark/services/auth/internal/constants"
)

var enrollLg = constants.GetLogger(constants.LoggerPrefixAuthService)

// A zero TTL would store the link without expiry, so only a positive value is taken.
func EnrollInviteTTL() time.Duration {
	seconds, err := getEnvAsInt64OrDefault(constants.EnvEnrollInviteTTLSec, constants.DefaultEnrollInviteTTLSec)
	if err != nil || seconds <= constants.DefaultInitValue {
		enrollLg.Warn(fmt.Sprintf(string(constants.LogEnvInvalid), constants.EnvEnrollInviteTTLSec,
			constants.DefaultEnrollInviteTTLSec))
		seconds = constants.DefaultEnrollInviteTTLSec
	}
	return time.Duration(seconds) * time.Second
}
