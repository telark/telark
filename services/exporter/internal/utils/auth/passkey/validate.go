package passkey

import (
	"net/http"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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
