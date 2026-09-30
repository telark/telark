package passkey

import (
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func GetPasskeyByCredentialID(w http.ResponseWriter, credentialID string, userID string) {
	resource, err := passkeyutils.FindPasskeyByCredentialIDAndUserID(credentialID, userID)
	if err != nil {
		sharedutils.LogDebugAndSend(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			err.Error(),
			nil,
			err,
		)
		return
	}

	passkeyMap, err := sharedutils.FilterData(resource)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(dataerrors.ErrFilterRes),
			nil,
			err,
		)
		return
	}

	msg := fmt.Sprintf(string(messages.SuccessGetRes), resource.GetName(), resource.GetKind())
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		passkeyMap,
		nil,
	)
}
