package status

import (
	"net/http"
	"strings"

	"github.com/telark/discovery/internal/constants"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	sharehelper "github.com/telark/discovery/internal/helpers/shared"
	kcoreconst "github.com/telark/kcore/constants"
	"github.com/telark/kcore/health"
	"github.com/telark/rest/response"
)

const (
	StatusField        = "status"
	DesiredField       = "desired"
	UnknownStatus      = "unknown"
	LastHeartbeatField = "last_heartbeat"
	HeartbeatZero      = 0
)

func ProbeHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, constants.StatusReadinessEp) && !readinessOK(r) {
		response.SendSingleResponse(
			w,
			response.NewGenericResponse(
				http.StatusServiceUnavailable,
				response.OperationUnprocessed,
				nil,
				sharehelper.ConcatWithColon("service is", "not ready"),
			),
		)
		return
	}

	response.SendSingleResponse(
		w,
		response.NewGenericResponse(
			http.StatusOK,
			response.OperationSuccess,
			nil,
			sharehelper.ConcatWithColon("service is", "healthy"),
		),
	)
}

func readinessOK(r *http.Request) bool {
	if !redishelper.IsBootstrapReady() {
		return false
	}
	return health.IsK8sReachable(r.Context(), kcoreconst.ZeroValue, kcoreconst.ZeroValue)
}
