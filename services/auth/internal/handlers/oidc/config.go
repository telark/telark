package oidc

import (
	"fmt"
	"net/http"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
)

// Validated here rather than at the exporter: reaching the provider and parsing its
// keys is this service's job, and a config that cannot authenticate anyone must not
// reach storage. The exporter's guard is bypassed on this hop because the call
// carries the service token, so the route requirement is what enforces Admin.
func SetConfig(w http.ResponseWriter, r *http.Request) {
	var req globalconfigresource.OIDCConfig
	if err := shared.DecodeRequestBody(r, &req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	if err := oidchelper.Validate(req); err != nil {
		shared.SendErrorResponse(w, http.StatusBadRequest, err)
		return
	}

	resp := clients.GetGlobalConfigClient().PatchGlobalConfig(map[string]any{
		globalconfigresource.FieldOIDC: req,
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

	lg.Info(string(constants.SuccessOIDCConfigUpdated))
	shared.SendSuccessResponse(w, string(constants.SuccessOIDCConfigUpdated), req)
}
