package session

import (
	"fmt"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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
