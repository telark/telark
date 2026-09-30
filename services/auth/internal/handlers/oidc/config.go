package oidc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/authz"
	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	telarkconfigresource "github.com/telark/data/resources/telarkconfig"
)

// Validated here, not at the exporter: a config that cannot authenticate anyone must not
// reach storage, and this hop carries the service token so the route requirement enforces Admin.
// Whoever controls the identity-provider trust can mint a login for any user, so
// Admin on the settings scope alone is not enough: the caller must be Admin everywhere.
func SetConfig(w http.ResponseWriter, r *http.Request) {
	if !authz.CallerIsAdminOnAll(r.Context()) {
		shared.SendErrorResponse(w, http.StatusForbidden, errors.New(string(constants.ErrOIDCConfigNeedsAdminAll)))
		return
	}

	var req SetConfigRequest
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}
	jwkGiven := req.GoogleJWKJSON != nil
	if jwkGiven {
		if err := json.Unmarshal(req.GoogleJWKJSON, &req.OIDCConfig.GoogleJWKJSON); err != nil {
			shared.SendErrorResponse(w, http.StatusBadRequest, fmt.Errorf(string(constants.ErrFailedDecodeRequest), err))
			return
		}
	}

	if err := oidchelper.Validate(req.OIDCConfig, jwkGiven); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	resp := clients.GetConfigClient().PatchConfig(map[string]any{
		telarkconfigresource.FieldOIDC: req,
	})
	if resp == nil || resp.Status != http.StatusOK {
		status := constants.DefaultInitValue
		if resp != nil {
			status = resp.Status
		}
		err := fmt.Errorf(string(constants.ErrOIDCConfigSaveFailed), status)
		shared.HandleError(w, err, http.StatusInternalServerError, err.Error())
		return
	}

	if jwkGiven {
		oidchelper.PinTrustJWK(req.OIDCConfig.GoogleJWKJSON)
	}

	lg.Info(string(constants.SuccessOIDCConfigUpdated))
	shared.SendSuccessResponse(w, string(constants.SuccessOIDCConfigUpdated), req.OIDCConfig)
}
