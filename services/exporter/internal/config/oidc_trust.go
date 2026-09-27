package config

import (
	"os"
	"strings"

	"github.com/telark/exporter/internal/constants"
)

func OIDCTrustSecretName() string {
	if name := strings.TrimSpace(os.Getenv(constants.EnvOIDCTrustSecretName)); name != constants.EmptyString {
		return name
	}
	return constants.DefaultOIDCTrustSecretName
}
