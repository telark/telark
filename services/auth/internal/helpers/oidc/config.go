package oidc

import (
	"errors"
	"fmt"
	"regexp"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/data/text"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
)

var googleClientID = regexp.MustCompile(constants.GoogleClientIDRegex)

// The exporter refuses the same values, but its refusal would reach the caller as a failed save (500).
// The trimmed client ID is what gets stored.
func CheckInputs(oidc *telarkconfigresource.OIDCConfig) error {
	if len(oidc.GoogleJWKJSON) > constants.OIDCJWKMaxBytes {
		return fmt.Errorf(string(constants.ErrOIDCJWKTooLarge), constants.OIDCJWKMaxBytes)
	}
	clientID, err := text.Check(oidc.GoogleClientID, text.Rule{
		Field: constants.RequestFieldGoogleClientID, Max: constants.GoogleClientIDMaxLen, Pattern: googleClientID,
	})
	if err != nil {
		return err
	}
	oidc.GoogleClientID = clientID
	return nil
}

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
// client and a reachable trust source; offline, an omitted (not cleared) set falls back to the mounted one.
func Validate(oidc telarkconfigresource.OIDCConfig, jwkGiven bool) error {
	if !oidc.Enabled {
		return nil
	}

	if oidc.GoogleClientID == constants.EmptyString {
		return errors.New(string(constants.ErrOIDCClientIDRequired))
	}

	jwkJSON := oidc.GoogleJWKJSON
	if !oidc.EgressAllowed && jwkJSON == constants.EmptyString {
		if !jwkGiven {
			jwkJSON = TrustJWK()
		}
		if jwkJSON == constants.EmptyString {
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
