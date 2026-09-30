package passkey

import (
	"net/http"

	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func ValidateLastPasskeyDeletion(w http.ResponseWriter, userID string) bool {
	passkeys, err := FindPasskeysByUserID(userID)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return false
	}

	if len(passkeys) <= constants.DefaultLastPasskeyCount {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrPasskeyCannotDeleteLast),
			nil,
			nil,
		)
		return false
	}

	return true
}
