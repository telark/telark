package applications

import (
	"net/http"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination"
	redishelper "github.com/telark/discovery/internal/helpers/redis"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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
