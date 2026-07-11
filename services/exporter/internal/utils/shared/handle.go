package shared

import (
	"net/http"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func HandleValidationError(w http.ResponseWriter, err error) {
	statusCode := http.StatusBadRequest
	errStr := err.Error()
	if errStr == string(constants.ErrChallengeExpired) || errStr == string(constants.ErrSessionExpired) {
		statusCode = http.StatusGone
	}
	responseutils.LogAndSendResponse(
		w,
		statusCode,
		response.OperationError,
		errStr,
		nil,
		nil,
	)
}
