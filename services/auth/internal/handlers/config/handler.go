package config

import (
	"net/http"

	authconfig "github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
)

// Unauthenticated, so it carries only what the login form needs; an unreachable
// config degrades to passkey-only rather than failing the page.
func GetConfig(w http.ResponseWriter, _ *http.Request) {
	data := map[string]any{
		constants.JSONKeySelfRegEnabled: authconfig.IsSelfRegistrationEnabled(),
		constants.JSONKeyOIDCEnabled:    false,
	}

	if oidc, err := oidchelper.LoadConfig(); err == nil && oidchelper.Usable(oidc) {
		data[constants.JSONKeyOIDCEnabled] = true
		data[constants.JSONKeyGoogleClientID] = oidc.GoogleClientID
	}

	shared.SendSuccessResponse(w, constants.ConfigMessage, data)
}
