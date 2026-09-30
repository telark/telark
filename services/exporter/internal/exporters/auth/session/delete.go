package session

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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

	deleteResult := api.DeleteCustomResourceByName(sessionName, authmetadata.SessionMetadata)
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
