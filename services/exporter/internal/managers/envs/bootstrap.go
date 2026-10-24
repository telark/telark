package envs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
)

var (
	bootstrapAdminEmail string
	breakGlassWait      = time.Duration(constants.DefaultBreakGlassWaitHours) * time.Hour
)

// The mailbox auth reserves for the bootstrap admin, compared trimmed and lowercased.
func InitBootstrapAdmin() {
	bootstrapAdminEmail = strings.ToLower(strings.TrimSpace(getEnv(constants.BootstrapAdminEnv)))
}

func GetBootstrapAdmin() string {
	return bootstrapAdminEmail
}

// Empty means the default; anything but a whole number of hours, at least 1, stops the exporter.
func InitBreakGlassWait() error {
	envVal := getEnv(constants.BreakGlassWaitHoursEnv)
	if envVal == constants.EmptyString {
		breakGlassWait = time.Duration(constants.DefaultBreakGlassWaitHours) * time.Hour
		return nil
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.DefaultIncrementValue {
		return fmt.Errorf(string(constants.ErrBreakGlassWaitInvalid), constants.BreakGlassWaitHoursEnv, envVal)
	}
	breakGlassWait = time.Duration(n) * time.Hour
	return nil
}

func GetBreakGlassWait() time.Duration {
	return breakGlassWait
}
