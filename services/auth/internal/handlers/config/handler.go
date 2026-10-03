package config

import (
	"fmt"
	"net/http"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/clients"
	authconfig "github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

// Unauthenticated, so it carries only what the login form needs, read from the
// cached config: page loads never reach the exporter more than once per TTL.
func GetConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := authconfig.TelarkConfig()
	data := map[string]any{
		constants.JSONKeySelfRegEnabled: cfg.SelfRegistration.Enabled,
		constants.JSONKeyOIDCEnabled:    false,
	}

	oidc := cfg.OIDC
	oidc.GoogleJWKJSON = oidchelper.TrustJWK()
	if oidchelper.Usable(oidc) {
		data[constants.JSONKeyOIDCEnabled] = true
		data[constants.JSONKeyGoogleClientID] = oidc.GoogleClientID
	}

	shared.SendSuccessResponse(w, constants.ConfigMessage, data)
}

// The exporter takes this setting from auth only, so the bootstrap check here is the whole gate.
func SetSelfRegistration(w http.ResponseWriter, r *http.Request) {
	if status, err := authz.GuardBootstrapCaller(r.Context()); err != nil {
		shared.SendErrorResponse(w, status, err)
		return
	}

	var req telarkconfigresource.SelfRegistrationConfig
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	resp := clients.GetConfigClient().PatchConfig(map[string]any{telarkconfigresource.FieldSelfRegistration: req})
	if resp.Status != http.StatusOK {
		err := fmt.Errorf(string(constants.ErrSelfRegistrationSaveFailed), resp.Status)
		shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
		return
	}

	lg.Info(string(constants.SuccessSelfRegistrationUpdated))
	shared.SendSuccessResponse(w, string(constants.SuccessSelfRegistrationUpdated), req)
}
