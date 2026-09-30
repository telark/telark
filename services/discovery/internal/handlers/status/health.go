package status

import (
	"context"
	"net/http"

	kcoreconst "github.com/telark/telark/internal/kcore/constants"
	"github.com/telark/telark/internal/kcore/health"
	statushandler "github.com/telark/telark/internal/rest/handlers/status"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/circuitbreaker"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
	"github.com/telark/telark/services/discovery/internal/startup"
)

var Liveness = statushandler.NewProbeHandler(
	constants.StatusReadinessEp,
	nil,
	statushandler.Messages{
		Ready:    constants.InfServiceHealthy,
		NotReady: constants.InfServiceNotReady,
	},
)

func Readiness(w http.ResponseWriter, r *http.Request) {
	// Only Redis gates the probe: it is the one dependency this replica cannot
	// serve without. Exporter, TelarkConfig and the apiserver fail cluster-wide,
	// so a 503 on them would empty the Service instead of serving degraded.
	reasons := degradedReasons(r.Context())
	if !redisReachable(r.Context()) {
		reasons = append(reasons, constants.ReadinessReasonRedis)
		respond(w, http.StatusServiceUnavailable, response.OperationUnprocessed, constants.InfServiceNotReady, reasons)
		return
	}
	respond(w, http.StatusOK, response.OperationSuccess, constants.InfServiceHealthy, reasons)
}

func respond(w http.ResponseWriter, status int, operation response.OperationStatus, message string, reasons []string) {
	data := Diagnostics{Degraded: len(reasons) > constants.DefaultInitValue, Reasons: reasons}
	response.SendSingleResponse(w, response.NewGenericResponse(status, operation, data, message))
}

func redisReachable(ctx context.Context) bool {
	rdb := redishelper.NewRedisClient()
	if rdb == nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, config.RedisPingTimeout())
	defer cancel()
	return rdb.Ping(pingCtx).Err() == nil
}

func degradedReasons(ctx context.Context) []string {
	var reasons []string
	if !redishelper.IsBootstrapReady() {
		reasons = append(reasons, constants.ReadinessReasonBootstrap)
	}
	if !startup.IsTelarkConfigReady() {
		reasons = append(reasons, constants.ReadinessReasonTelarkConfig)
	}
	if circuitbreaker.GetManager().GetState(circuitbreaker.DependencyExporter) == circuitbreaker.StateOpen {
		reasons = append(reasons, constants.ReadinessReasonExporter)
	}
	if !health.IsK8sReachable(ctx, kcoreconst.ZeroValue, kcoreconst.ZeroValue) {
		reasons = append(reasons, constants.ReadinessReasonKubernetes)
	}
	return reasons
}
