package config

import "github.com/telark/auth/internal/constants"

func OIDCTrustFile() string {
	return getEnvOrDefault(constants.EnvOIDCTrustFile, constants.DefaultOIDCTrustFile)
}
