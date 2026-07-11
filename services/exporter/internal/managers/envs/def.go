package envs

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/telark/exporter/internal/constants"
)

func LoadAndValidateEnv(envVarName string) (string, error) {
	allowedEnvVarPattern := regexp.MustCompile(constants.AllowedEnvVarPattern)
	if !allowedEnvVarPattern.MatchString(envVarName) {
		return "", fmt.Errorf(string(constants.ErrInvalidEnvVarName), envVarName)
	}

	env := os.Getenv(envVarName)

	if env == constants.EmptyString {
		return "", fmt.Errorf(string(constants.ErrEnvVarNotSet), envVarName)
	}

	if len(env) > constants.MaxEnvVarLength {
		return constants.EmptyString, fmt.Errorf(
			string(constants.ErrEnvVarExceedsMaxLength), envVarName, constants.MaxEnvVarLength)
	}

	sanitizedEnv := sanitizeEnvValue(env)
	if sanitizedEnv == constants.EmptyString {
		return constants.EmptyString, fmt.Errorf(
			string(constants.ErrEnvVarContainsInvalidCharacters), envVarName)
	}

	return sanitizedEnv, nil
}

func sanitizeEnvValue(value string) string {
	clean := strings.Map(func(r rune) rune {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return -1
		}
		return r
	}, value)

	clean = strings.TrimSpace(clean)

	return clean
}
