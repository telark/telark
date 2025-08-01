package shared

import (
	"errors"
	"os"
	"strconv"
)

type EnvConfig struct {
	Key          string
	DefaultValue string
	Required     bool
	ErrorMsg     string
}

func GetEnvString(config EnvConfig) (string, error) {
	value := os.Getenv(config.Key)
	if value == "" {
		if config.Required {
			return "", errors.New(config.ErrorMsg)
		}
		return config.DefaultValue, nil
	}
	return value, nil
}

func GetEnvInt(config EnvConfig) (int, error) {
	value := os.Getenv(config.Key)
	if value == "" {
		if config.Required {
			return 0, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != "" {
			return strconv.Atoi(config.DefaultValue)
		}
		return 0, nil
	}
	return strconv.Atoi(value)
}

func GetEnvBool(config EnvConfig) (bool, error) {
	value := os.Getenv(config.Key)
	if value == "" {
		if config.Required {
			return false, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != "" {
			return strconv.ParseBool(config.DefaultValue)
		}
		return false, nil
	}
	return strconv.ParseBool(value)
}
