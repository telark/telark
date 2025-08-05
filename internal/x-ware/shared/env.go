package shared

import (
	"errors"
	"os"
	"strconv"
)

const (
	emptyString = ""
	defaultInt  = 0
)

type EnvConfig struct {
	Key          string
	DefaultValue string
	Required     bool
	ErrorMsg     string
}

func GetEnvString(config EnvConfig) (string, error) {
	value := os.Getenv(config.Key)
	if value == emptyString {
		if config.Required {
			return emptyString, errors.New(config.ErrorMsg)
		}
		return config.DefaultValue, nil
	}
	return value, nil
}

func GetEnvInt(config EnvConfig) (int, error) {
	value := os.Getenv(config.Key)
	if value == emptyString {
		if config.Required {
			return defaultInt, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != emptyString {
			return strconv.Atoi(config.DefaultValue)
		}
		return defaultInt, nil
	}
	return strconv.Atoi(value)
}

func GetEnvBool(config EnvConfig) (bool, error) {
	value := os.Getenv(config.Key)
	if value == emptyString {
		if config.Required {
			return false, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != emptyString {
			return strconv.ParseBool(config.DefaultValue)
		}
		return false, nil
	}
	return strconv.ParseBool(value)
}
