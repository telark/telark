package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

var lg = constants.GetLogger(constants.PrefixShared)

// responseutils logs at Error whenever err is non-nil, so a client-side failure
// is reported here and the error is withheld from that call.
func LogByStatusAndSend(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) {
	if err != nil && status < http.StatusInternalServerError {
		lg.Warn(fmt.Sprintf(constants.LogMessageWithError, message, err))
		err = nil
	}
	responseutils.LogAndSendResponse(w, status, operation, message, data, err)
}

func LogDebugAndSend(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) {
	if err != nil {
		lg.Debug(fmt.Sprintf(constants.LogMessageWithError, message, err))
	}
	responseutils.LogAndSendResponse(w, status, operation, message, data, nil)
}
