package shared

import (
	"errors"
	"os"
	"strconv"

	"github.com/telark/x-ware/constants"
)

type EnvConfig struct {
	Key          string
	DefaultValue string
	Required     bool
	ErrorMsg     string
}

func GetEnvString(config EnvConfig) (string, error) {
	value := os.Getenv(config.Key)
	if value == constants.EmptyString {
		if config.Required {
			return constants.EmptyString, errors.New(config.ErrorMsg)
		}
		return config.DefaultValue, nil
	}
	return value, nil
}

func GetEnvInt(config EnvConfig) (int, error) {
	value := os.Getenv(config.Key)
	if value == constants.EmptyString {
		if config.Required {
			return constants.ZeroValue, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != constants.EmptyString {
			return strconv.Atoi(config.DefaultValue)
		}
		return constants.ZeroValue, nil
	}
	return strconv.Atoi(value)
}

func GetEnvBool(config EnvConfig) (bool, error) {
	value := os.Getenv(config.Key)
	if value == constants.EmptyString {
		if config.Required {
			return false, errors.New(config.ErrorMsg)
		}
		if config.DefaultValue != constants.EmptyString {
			return strconv.ParseBool(config.DefaultValue)
		}
		return false, nil
	}
	return strconv.ParseBool(value)
}
