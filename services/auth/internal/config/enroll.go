package config

import (
	"fmt"
	"os"
	"strings"
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

// The chart pins the passkey origin whenever it exposes the dashboard, so empty means no public address to link to.
func DashboardOrigin() string {
	origin, _, _ := strings.Cut(os.Getenv(constants.EnvRPOrigin), constants.CommaSeparator)
	return strings.TrimSpace(origin)
}
