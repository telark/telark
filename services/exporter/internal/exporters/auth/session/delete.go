package session

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	authmetadata "github.com/telark/data/metadata/auth"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func DeleteSessionByToken(w http.ResponseWriter, token string) {
	resource, ok := sessionutils.FindSessionOrRespond(w, token)
	if !ok {
		return
	}

	sessionName := resource.GetName()

	lock := concurrency.GetLock(sessionName)
	lock.Lock()
	defer lock.Unlock()

	deleteResult := api.DeleteCustomResourceByName(sessionName, authmetadata.UserSessionMetadata)
	if deleteResult.Status != http.StatusOK {
		errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, sessionName, deleteResult.Error)
		responseutils.LogAndSendResponse(
			w,
			deleteResult.Status,
			response.OperationError,
			errorMsg,
			nil,
			deleteResult.Error,
		)
		return
	}

	msg := fmt.Sprintf(string(messages.SuccessDeleteRes), resource.GetName(), resource.GetKind())
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		nil,
		nil,
	)
}
