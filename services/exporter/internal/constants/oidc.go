package constants

import (
	"time"

	"github.com/telark/data/errors"
)

const (
	EnvOIDCTrustSecretName     = "OIDC_TRUST_SECRET_NAME"
	DefaultOIDCTrustSecretName = "telark-oidc-trust-secret"
	SecretDataField            = "data"
	SecretVersion              = "v1"
	SecretResource             = "secrets"
	OIDCTrustTimeout           = 10 * time.Second
)

const (
	ErrOIDCTrustSecretClient errors.Error = "OIDC trust anchor: kubernetes client unavailable: %v"
	ErrOIDCTrustSecretRead   errors.Error = "failed to read the OIDC trust anchor from %s: %v"
	ErrOIDCTrustSecretWrite  errors.Error = "failed to write the OIDC trust anchor to %s: %v"
	ErrOIDCTrustSecretDecode errors.Error = "the OIDC trust anchor in %s (key %s) is not valid base64: %v"
	ErrOIDCJWKNotString      errors.Error = "oidc.googleJwkJson must be a string"
	WarnOIDCTrustMergeFailed errors.Error = "config view served without oidc.googleJwkJson: %v"
)
