package oidc

import (
	"errors"
	"fmt"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
)

func LoadConfig() (globalconfigresource.OIDCConfig, error) {
	cfg, err := clients.GetGlobalConfigClient().GetGlobalConfig()
	if err != nil {
		return globalconfigresource.OIDCConfig{}, fmt.Errorf(string(constants.ErrOIDCConfigLoadFailed), err)
	}
	return cfg.OIDC, nil
}

func Usable(oidc globalconfigresource.OIDCConfig) bool {
	if !oidc.Enabled || oidc.GoogleClientID == constants.EmptyString {
		return false
	}
	return oidc.EgressAllowed || oidc.GoogleJWKJSON != constants.EmptyString
}

// Rejects a config that would break login before it is stored: a disabled block
// needs no checks, an enabled one must name a client and a reachable trust source.
func Validate(oidc globalconfigresource.OIDCConfig) error {
	if !oidc.Enabled {
		return nil
	}

	if oidc.GoogleClientID == constants.EmptyString {
		return errors.New(string(constants.ErrOIDCClientIDRequired))
	}

	if !oidc.EgressAllowed && oidc.GoogleJWKJSON == constants.EmptyString {
		return errors.New(string(constants.ErrOIDCTrustSourceRequired))
	}

	if oidc.GoogleJWKJSON != constants.EmptyString {
		if _, err := parseKeys([]byte(oidc.GoogleJWKJSON)); err != nil {
			return err
		}
		return nil
	}

	raw, err := fetchJWKS()
	if err != nil {
		return fmt.Errorf(string(constants.ErrOIDCJWKUnreachable), err)
	}
	_, err = parseKeys(raw)
	return err
}
