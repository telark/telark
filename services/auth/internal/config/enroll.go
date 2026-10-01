package config

import (
	"time"

	"github.com/telark/telark/services/auth/internal/constants"
)

// A zero TTL would store the link without expiry, so only a positive value is taken.
func EnrollInviteTTL() time.Duration {
	return envSeconds(constants.EnvEnrollInviteTTLSec, constants.DefaultEnrollInviteTTLSec)
}
