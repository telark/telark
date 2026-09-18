package session

import (
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GetSessionByToken(w http.ResponseWriter, token string) {
	resource, ok := sessionutils.FindSessionOrRespond(w, token)
	if !ok {
		return
	}

	session, err := sessionutils.ValidateSessionExpiration(resource)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}

	sessionMap, err := sharedutils.StructToSpecMap(session)
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
		sessionMap,
		nil,
	)
}
