package oidc

import (
	"encoding/json"
	"fmt"
	"net/http"

	telarkconfigresource "github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/auth/internal/authz"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

// Validated here, not at the exporter: a config that cannot authenticate anyone must not
// reach storage, and this hop carries the service token so the route requirement enforces Admin.
// Whoever controls the identity-provider trust can mint a login for any user, so
// only the bootstrap account may change it, whatever grants the caller holds.
func SetConfig(w http.ResponseWriter, r *http.Request) {
	if status, err := authz.GuardBootstrapCaller(r.Context()); err != nil {
		shared.SendErrorResponse(w, status, err)
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
