package config

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/telark/auth/internal/constants"
)

type BootstrapConfig struct {
	BootstrapAdmins         []string
	SelfRegistrationEnabled bool
}

var (
	globalBootstrapConfig *BootstrapConfig
	bootstrapMutex        sync.RWMutex
)

func LoadBootstrapConfig() (*BootstrapConfig, error) {
	admins := parseBootstrapAdmins(getEnvOrDefault(constants.EnvBootstrapAdmins, constants.EmptyString))
	selfRegEnabled := getEnvAsBool(constants.EnvSelfRegistrationEnabled, true)

	cfg := &BootstrapConfig{
		BootstrapAdmins:         admins,
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
	normalizedEmail := strings.ToLower(email)
	return slices.Contains(cfg.BootstrapAdmins, normalizedEmail)
}

func IsSelfRegistrationEnabled() bool {
	cfg := GetBootstrapConfig()
	if cfg == nil {
		return true
	}
	return cfg.SelfRegistrationEnabled
}

func parseBootstrapAdmins(raw string) []string {
	if raw == constants.EmptyString {
		return nil
	}

	entries := strings.Split(raw, ",")
	admins := make([]string, constants.InitialCapacity, len(entries))

	for _, entry := range entries {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == constants.EmptyString {
			continue
		}
		admins = append(admins, entry)
	}

	return admins
}

func validateBootstrapConfig(cfg *BootstrapConfig) error {
	if len(cfg.BootstrapAdmins) == constants.DefaultInitValue && !cfg.SelfRegistrationEnabled {
		return fmt.Errorf("%s", constants.ErrBootstrapNoAdminsAndNoSelfReg)
	}
	return nil
}
