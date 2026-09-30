package status

import (
	"context"
	"net/http"

	kcoreconst "github.com/telark/telark/internal/kcore/constants"
	"github.com/telark/telark/internal/kcore/health"
	statuseps "github.com/telark/telark/internal/rest/endpoints/status"
	statushandler "github.com/telark/telark/internal/rest/handlers/status"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
)

var Liveness = statushandler.NewProbeHandler(
	string(statuseps.LivenessCheck),
	nil,
	statushandler.Messages{
		Ready:    string(constants.InfSuccessStatusMessage),
		NotReady: string(constants.InfNotReadyStatusMessage),
	},
)

// Only this replica's own dependencies gate the probe: Redis and the watch
// cache. The apiserver fails cluster-wide, so a slow ServerVersion call is
// reported as degraded rather than emptying the Service.
func Readiness(w http.ResponseWriter, r *http.Request) {
	reasons := degradedReasons(r.Context())
	if blocking := notReadyReasons(r.Context()); len(blocking) > constants.DefaultInitValue {
		respond(w, http.StatusServiceUnavailable, response.OperationUnprocessed,
			string(constants.InfNotReadyStatusMessage), append(blocking, reasons...))
		return
	}
	respond(w, http.StatusOK, response.OperationSuccess, string(constants.InfSuccessStatusMessage), reasons)
}

func respond(w http.ResponseWriter, status int, operation response.OperationStatus, message string, reasons []string) {
	data := Diagnostics{Degraded: len(reasons) > constants.DefaultInitValue, Reasons: reasons}
	response.SendSingleResponse(w, response.NewGenericResponse(status, operation, data, message))
}

func notReadyReasons(ctx context.Context) []string {
	var reasons []string
	if !redisReachable(ctx) {
		reasons = append(reasons, constants.ReadinessReasonRedis)
	}
	if !informers.ApplicationsSynced() {
		reasons = append(reasons, constants.ReadinessReasonInformer)
	}
	return reasons
}

func degradedReasons(ctx context.Context) []string {
	if health.IsK8sReachable(ctx, kcoreconst.ZeroValue, kcoreconst.ZeroValue) {
		return nil
	}
	return []string{constants.ReadinessReasonKubernetes}
}

func redisReachable(ctx context.Context) bool {
	rdb := exprdb.Get()
	if rdb == nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, constants.ReadinessPingTimeout)
	defer cancel()
	return rdb.Ping(pingCtx).Err() == nil
}
