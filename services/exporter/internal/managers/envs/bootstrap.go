package envs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
)

var (
	bootstrapAdminEmail  string
	authRequestWait      = time.Duration(constants.DefaultAuthRequestWaitHours) * time.Hour
	authRequestRetention = constants.DefaultAuthRequestRetentionDays * constants.AuthRequestDay
)

// The mailbox auth reserves for the bootstrap admin, compared trimmed and lowercased.
func InitBootstrapAdmin() {
	bootstrapAdminEmail = strings.ToLower(strings.TrimSpace(getEnv(constants.BootstrapAdminEnv)))
}

func GetBootstrapAdmin() string {
	return bootstrapAdminEmail
}

// Empty means the default; anything but a whole number of hours, at least 1, stops the exporter.
func InitAuthRequestWait() error {
	envVal := getEnv(constants.AuthRequestWaitHoursEnv)
	if envVal == constants.EmptyString {
		authRequestWait = time.Duration(constants.DefaultAuthRequestWaitHours) * time.Hour
		return nil
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.DefaultIncrementValue {
		return fmt.Errorf(string(constants.ErrAuthRequestWaitInvalid), constants.AuthRequestWaitHoursEnv, envVal)
	}
	authRequestWait = time.Duration(n) * time.Hour
	return nil
}

func GetAuthRequestWait() time.Duration {
	return authRequestWait
}

// Empty means the default; anything but a whole number of days, at least 1, stops the exporter.
func InitAuthRequestRetention() error {
	envVal := getEnv(constants.AuthRequestRetentionDaysEnv)
	if envVal == constants.EmptyString {
		authRequestRetention = constants.DefaultAuthRequestRetentionDays * constants.AuthRequestDay
		return nil
	}
	n, err := strconv.Atoi(envVal)
	if err != nil || n < constants.DefaultIncrementValue {
		return fmt.Errorf(string(constants.ErrAuthRequestRetentionInvalid), constants.AuthRequestRetentionDaysEnv, envVal)
	}
	authRequestRetention = time.Duration(n) * constants.AuthRequestDay
	return nil
}

func GetAuthRequestRetention() time.Duration {
	return authRequestRetention
}
