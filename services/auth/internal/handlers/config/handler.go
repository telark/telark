package config

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
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

func SetSelfRegistration(w http.ResponseWriter, r *http.Request) {
	setBootstrapSetting[telarkconfigresource.SelfRegistrationConfig](w, r, telarkconfigresource.FieldSelfRegistration,
		constants.ErrSelfRegistrationSaveFailed, constants.SuccessSelfRegistrationUpdated)
}

func SetBreakGlassAlerts(w http.ResponseWriter, r *http.Request) {
	setBootstrapSetting[telarkconfigresource.BreakGlassAlertsConfig](w, r, telarkconfigresource.FieldBreakGlassAlerts,
		constants.ErrBreakGlassAlertsSaveFailed, constants.SuccessBreakGlassAlertsUpdated)
}

// The exporter takes these settings from auth only, so the bootstrap check here is the whole gate.
func setBootstrapSetting[T any](w http.ResponseWriter, r *http.Request, field string, failed errors.Error, saved messages.Message) {
	if status, err := authz.GuardBootstrapCaller(r.Context()); err != nil {
		shared.SendErrorResponse(w, status, err)
		return
	}

	var req T
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	resp := clients.GetConfigClient().PatchConfig(map[string]any{field: req})
	if resp.Status != http.StatusOK {
		err := fmt.Errorf(string(failed), resp.Status)
		shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
		return
	}

	lg.Info(string(saved))
	shared.SendSuccessResponse(w, string(saved), req)
}
