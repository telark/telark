package status

import (
	"net/http"

	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/helpers/shared"
)

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	shared.SendSuccessResponse(w, constants.HealthMessageHealthy, map[string]any{
		constants.JSONKeyStatus:  constants.HealthStatusOK,
		constants.JSONKeyService: constants.ServiceName,
	})
}

func ReadinessHandler(w http.ResponseWriter, _ *http.Request) {
	shared.SendSuccessResponse(w, constants.HealthMessageReady, map[string]any{
		constants.JSONKeyStatus:  constants.HealthStatusReady,
		constants.JSONKeyService: constants.ServiceName,
	})
}

func LivenessHandler(w http.ResponseWriter, _ *http.Request) {
	shared.SendSuccessResponse(w, constants.HealthMessageAlive, map[string]any{
		constants.JSONKeyStatus:  constants.HealthStatusAlive,
		constants.JSONKeyService: constants.ServiceName,
	})
}
