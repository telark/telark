package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/telark/auth/internal/constants"
)

type Config struct {
	Service  ServiceConfig
	WebAuthn WebAuthnConfig
}

type ServiceConfig struct {
	Port              string
	ShutdownTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

type WebAuthnConfig struct {
	RPID             string
	RPName           string
	RPOrigin         string
	ChallengeTimeout int64
	SessionExpiry    int64
}

var (
	globalConfig *Config
	configOnce   sync.Once
	configMutex  sync.RWMutex
)

func LoadConfig() (*Config, error) {
	configMutex.RLock()
	if globalConfig != nil {
		cfg := globalConfig
		configMutex.RUnlock()
		return cfg, nil
	}
	configMutex.RUnlock()

	var err error
	configOnce.Do(func() {
		var cfg *Config
		cfg, err = loadConfigInternal()
		if err == nil {
			configMutex.Lock()
			globalConfig = cfg
			configMutex.Unlock()
		}
	})

	if err != nil {
		return nil, err
	}

	configMutex.RLock()
	cfg := globalConfig
	configMutex.RUnlock()
	return cfg, nil
}

func loadConfigInternal() (*Config, error) {
	rpID, err := getEnvOrFail("RP_ID")
	if err != nil {
		return nil, err
	}

	rpName, err := getEnvOrFail("RP_NAME")
	if err != nil {
		return nil, err
	}

	rpOrigin, err := getEnvOrFail("RP_ORIGIN")
	if err != nil {
		return nil, err
	}

	challengeTimeout, err := getEnvAsInt64OrDefault(
		"CHALLENGE_TIMEOUT",
		constants.DefaultChallengeTimeout,
	)
	if err != nil {
		return nil, err
	}

	sessionExpiry, err := getEnvAsInt64OrDefault(
		"SESSION_EXPIRY",
		constants.DefaultSessionExpiry,
	)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Service: ServiceConfig{
			Port:              getEnvOrDefault("PORT", constants.DefaultPort),
			ShutdownTimeout:   constants.DefaultShutdownTimeout,
			ReadHeaderTimeout: constants.DefaultReadHeaderTimeout,
			ReadTimeout:       constants.DefaultReadTimeout,
			WriteTimeout:      constants.DefaultWriteTimeout,
			IdleTimeout:       constants.DefaultIdleTimeout,
		},
		WebAuthn: WebAuthnConfig{
			RPID:             rpID,
			RPName:           rpName,
			RPOrigin:         rpOrigin,
			ChallengeTimeout: challengeTimeout,
			SessionExpiry:    sessionExpiry,
		},
	}

	return cfg, nil
}

func GetConfig() (*Config, error) {
	configMutex.RLock()
	if globalConfig != nil {
		cfg := globalConfig
		configMutex.RUnlock()
		return cfg, nil
	}
	configMutex.RUnlock()

	return LoadConfig()
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != constants.EmptyString {
		return value
	}
	return defaultValue
}

func getEnvOrFail(key string) (string, error) {
	value := os.Getenv(key)
	if value == constants.EmptyString {
		return constants.EmptyString, fmt.Errorf(string(constants.ErrMissingEnvVar), key)
	}
	return value, nil
}

const (
	emptyString = constants.EmptyString
	base10      = 10
	int64Bits   = 64
)

func getEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == emptyString {
		return defaultValue
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return b
}

func getEnvAsInt64OrDefault(key string, defaultValue int64) (int64, error) {
	valueStr := os.Getenv(key)
	if valueStr == emptyString {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(valueStr, base10, int64Bits)
	if err != nil {
		return constants.InitialCapacity, fmt.Errorf(string(constants.ErrInvalidEnvVarValue), key, valueStr)
	}

	return value, nil
}
