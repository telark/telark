package config

import (
	"net/http"

	authconfig "github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
)

// Read before any session exists, so it carries only what the browser needs to draw
// the login form. It stays answerable when the config is unreachable: the page falls
// back to passkey login rather than failing to render.
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
