package config

import (
	"errors"
	"strings"
	"sync"

	"github.com/telark/telark/services/auth/internal/constants"
)

type BootstrapConfig struct {
	BootstrapAdmin          string
	SelfRegistrationEnabled bool
}

var (
	globalBootstrapConfig *BootstrapConfig
	bootstrapMutex        sync.RWMutex
)

func LoadBootstrapConfig() (*BootstrapConfig, error) {
	admin := normalizeEmail(getEnvOrDefault(constants.EnvBootstrapAdmin, constants.EmptyString))
	selfRegEnabled := getEnvAsBool(constants.EnvSelfRegistrationEnabled, constants.DefaultSelfRegistrationEnabled)

	cfg := &BootstrapConfig{
		BootstrapAdmin:          admin,
		SelfRegistrationEnabled: selfRegEnabled,
	}

	if err := validateBootstrapConfig(cfg); err != nil {
		return nil, err
	}

	bootstrapMutex.Lock()
	globalBootstrapConfig = cfg
	bootstrapMutex.Unlock()

	return cfg, nil
}

func GetBootstrapConfig() *BootstrapConfig {
	bootstrapMutex.RLock()
	cfg := globalBootstrapConfig
	bootstrapMutex.RUnlock()
	return cfg
}

func IsBootstrapAdmin(email string) bool {
	cfg := GetBootstrapConfig()
	if cfg == nil {
		return false
	}
	return cfg.BootstrapAdmin != constants.EmptyString && normalizeEmail(email) == cfg.BootstrapAdmin
}

func IsSelfRegistrationEnabled() bool {
	cfg := GetBootstrapConfig()
	if cfg == nil {
		return constants.DefaultSelfRegistrationEnabled
	}
	return cfg.SelfRegistrationEnabled
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateBootstrapConfig(cfg *BootstrapConfig) error {
	if cfg.BootstrapAdmin == constants.EmptyString && !cfg.SelfRegistrationEnabled {
		return errors.New(string(constants.ErrBootstrapNoAdminAndNoSelfReg))
	}
	return nil
}
