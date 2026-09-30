package config

import "github.com/telark/telark/services/auth/internal/constants"

func OIDCTrustFile() string {
	return getEnvOrDefault(constants.EnvOIDCTrustFile, constants.DefaultOIDCTrustFile)
}
