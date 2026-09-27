package oidc

import (
	"errors"
	"fmt"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	telarkconfigresource "github.com/telark/data/resources/telarkconfig"
)

func LoadConfig() (telarkconfigresource.OIDCConfig, error) {
	cfg, err := clients.GetConfigClient().GetConfig()
	if err != nil {
		return telarkconfigresource.OIDCConfig{}, fmt.Errorf(string(constants.ErrOIDCConfigLoadFailed), err)
	}
	// The pinned set lives in the mounted trust Secret, never in the config the exporter returns.
	oidc := cfg.OIDC
	oidc.GoogleJWKJSON = TrustJWK()
	return oidc, nil
}

func Usable(oidc telarkconfigresource.OIDCConfig) bool {
	if !oidc.Enabled || oidc.GoogleClientID == constants.EmptyString {
		return false
	}
	return oidc.EgressAllowed || oidc.GoogleJWKJSON != constants.EmptyString
}

// Rejects a config that would break login before it is stored: an enabled one must name a
// client and a reachable trust source; offline, an omitted set falls back to the mounted one.
func Validate(oidc telarkconfigresource.OIDCConfig) error {
	if !oidc.Enabled {
		return nil
	}

	if oidc.GoogleClientID == constants.EmptyString {
		return errors.New(string(constants.ErrOIDCClientIDRequired))
	}

	jwkJSON := oidc.GoogleJWKJSON
	if !oidc.EgressAllowed && jwkJSON == constants.EmptyString {
		if jwkJSON = TrustJWK(); jwkJSON == constants.EmptyString {
			return errors.New(string(constants.ErrOIDCTrustSourceRequired))
		}
	}

	if jwkJSON != constants.EmptyString {
		_, err := parseKeys([]byte(jwkJSON))
		return err
	}

	raw, err := fetchJWKS()
	if err != nil {
		return fmt.Errorf(string(constants.ErrOIDCJWKUnreachable), err)
	}
	_, err = parseKeys(raw)
	return err
}
