package status

import (
	"net/http"

	"github.com/telark/discovery/internal/constants"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	kcoreconst "github.com/telark/kcore/constants"
	"github.com/telark/kcore/health"
	statushandler "github.com/telark/rest/handlers/status"
)

const (
	StatusField        = "status"
	DesiredField       = "desired"
	UnknownStatus      = "unknown"
	LastHeartbeatField = "last_heartbeat"
	HeartbeatZero      = 0
)

var ProbeHandler = statushandler.NewProbeHandler(
	constants.StatusReadinessEp,
	readinessOK,
	statushandler.Messages{
		Ready:    constants.InfServiceHealthy,
		NotReady: constants.InfServiceNotReady,
	},
)

func readinessOK(r *http.Request) bool {
	if !redishelper.IsBootstrapReady() {
		return false
	}
	return health.IsK8sReachable(r.Context(), kcoreconst.ZeroValue, kcoreconst.ZeroValue)
}
