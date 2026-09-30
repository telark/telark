package shared

import (
	"net/http"

	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func HandleValidationError(w http.ResponseWriter, err error) {
	statusCode := http.StatusBadRequest
	errStr := err.Error()
	if errStr == string(constants.ErrChallengeExpired) || errStr == string(constants.ErrSessionExpired) {
		statusCode = http.StatusGone
	}
	if errStr == string(constants.ErrPasskeyNotFound) {
		statusCode = http.StatusNotFound
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
