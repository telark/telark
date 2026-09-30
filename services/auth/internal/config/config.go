package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/telark/telark/services/auth/internal/constants"
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
	globalConfig    *Config
	globalConfigErr error
	configOnce      sync.Once
)

// sync.Once publishes both results to every later caller, so a first load that
// failed keeps reporting its error instead of handing back a nil config.
func LoadConfig() (*Config, error) {
	configOnce.Do(func() {
		globalConfig, globalConfigErr = loadConfigInternal()
	})
	return globalConfig, globalConfigErr
}

func loadConfigInternal() (*Config, error) {
	rpName, err := getEnvOrFail(constants.EnvRPName)
	if err != nil {
		return nil, err
	}

	challengeTimeout, err := getEnvAsInt64OrDefault(
		constants.EnvChallengeTimeout,
		constants.DefaultChallengeTimeout,
	)
	if err != nil {
		return nil, err
	}

	sessionExpiry, err := getEnvAsInt64OrDefault(
		constants.EnvSessionExpiry,
		constants.DefaultSessionExpiry,
	)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Service: ServiceConfig{
			Port:              getEnvOrDefault(constants.EnvPort, constants.DefaultPort),
			ShutdownTimeout:   constants.DefaultShutdownTimeout,
			ReadHeaderTimeout: constants.DefaultReadHeaderTimeout,
			ReadTimeout:       constants.DefaultReadTimeout,
			WriteTimeout:      constants.DefaultWriteTimeout,
			IdleTimeout:       constants.DefaultIdleTimeout,
		},
		WebAuthn: WebAuthnConfig{
			RPID:             os.Getenv(constants.EnvRPID),
			RPName:           rpName,
			RPOrigin:         os.Getenv(constants.EnvRPOrigin),
			ChallengeTimeout: challengeTimeout,
			SessionExpiry:    sessionExpiry,
		},
	}

	return cfg, nil
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
	base10    = 10
	int64Bits = 64
)

func getEnvAsBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == constants.EmptyString {
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
	if valueStr == constants.EmptyString {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(valueStr, base10, int64Bits)
	if err != nil {
		return constants.InitialCapacity, fmt.Errorf(string(constants.ErrInvalidEnvVarValue), key, valueStr)
	}

	return value, nil
}
