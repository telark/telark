package config

import (
	"net/http"

	authconfig "github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
)

func GetConfig(w http.ResponseWriter, _ *http.Request) {
	shared.SendSuccessResponse(w, constants.ConfigMessage, map[string]any{
		constants.JSONKeySelfRegEnabled: authconfig.IsSelfRegistrationEnabled(),
	})
}
