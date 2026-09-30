package passkey

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func DeletePasskeyByCredentialID(w http.ResponseWriter, credentialID string, userID string, forceLastDelete bool) {
	resource, err := passkeyutils.FindPasskeyByCredentialIDAndUserID(credentialID, userID)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}
	passkeyName := resource.GetName()

	if !forceLastDelete {
		if !passkeyutils.ValidateLastPasskeyDeletion(w, userID) {
			return
		}
	}

	lock := concurrency.GetLock(passkeyName)
	lock.Lock()
	defer lock.Unlock()

	deleteResult := api.DeleteCustomResourceByName(passkeyName, authmetadata.PasskeyMetadata)
	if deleteResult.Status != http.StatusOK {
		errorMsg := sharedutils.GenerateResourceError(errors.ErrDeleteRes, passkeyName, deleteResult.Error)
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
