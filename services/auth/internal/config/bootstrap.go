package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/telark/telark/services/auth/internal/constants"
)

type BootstrapConfig struct {
	BootstrapAdmin string
}

var (
	globalBootstrapConfig *BootstrapConfig
	breakGlassWait        = time.Duration(constants.DefaultBreakGlassWaitHours) * time.Hour
	bootstrapMutex        sync.RWMutex
)

func LoadBootstrapConfig() (*BootstrapConfig, error) {
	admin := normalizeEmail(getEnvOrDefault(constants.EnvBootstrapAdmin, constants.EmptyString))

	cfg := &BootstrapConfig{
		BootstrapAdmin: admin,
	}

	if err := validateBootstrapConfig(cfg); err != nil {
		return nil, err
	}
	hours, err := getEnvAsInt64OrDefault(constants.EnvBreakGlassWaitHours, constants.DefaultBreakGlassWaitHours)
	if err != nil || hours < constants.DefaultIncrementValue {
		return nil, fmt.Errorf(string(constants.ErrBreakGlassWaitInvalid), constants.EnvBreakGlassWaitHours,
			os.Getenv(constants.EnvBreakGlassWaitHours))
	}

	bootstrapMutex.Lock()
	globalBootstrapConfig = cfg
	breakGlassWait = time.Duration(hours) * time.Hour
	bootstrapMutex.Unlock()

	return cfg, nil
}

func GetBootstrapConfig() *BootstrapConfig {
	bootstrapMutex.RLock()
	cfg := globalBootstrapConfig
	bootstrapMutex.RUnlock()
	return cfg
}

func BreakGlassWait() time.Duration {
	bootstrapMutex.RLock()
	defer bootstrapMutex.RUnlock()
	return breakGlassWait
}

func IsBootstrapAdmin(email string) bool {
	cfg := GetBootstrapConfig()
	if cfg == nil {
		return false
	}
	return cfg.BootstrapAdmin != constants.EmptyString && normalizeEmail(email) == cfg.BootstrapAdmin
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// SSO and self-registration start off and only the bootstrap account may turn them
// on, so an install without one could never open either.
func validateBootstrapConfig(cfg *BootstrapConfig) error {
	if cfg.BootstrapAdmin == constants.EmptyString {
		return errors.New(string(constants.ErrBootstrapAdminRequired))
	}
	return nil
}
