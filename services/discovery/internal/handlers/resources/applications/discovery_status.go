package applications

import (
	"net/http"

	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
	redishelper "github.com/telark/telark/services/discovery/internal/helpers/redis"
)

func DiscoveryStatus(w http.ResponseWriter, r *http.Request) {
	status := coordination.CycleStatus(r.Context(), redishelper.NewRedisClient())
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(constants.MsgDiscoveryStatusFetched),
		status,
		nil,
	)
}
