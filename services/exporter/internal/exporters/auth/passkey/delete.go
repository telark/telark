package passkey

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	authmetadata "github.com/telark/data/metadata/auth"
	passkeyutils "github.com/telark/exporter/internal/utils/auth/passkey"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

//nolint:revive // forceLastDelete should be used to delete the last passkey
func DeletePasskeyByCredentialID(w http.ResponseWriter, credentialID string, userID string, forceLastDelete bool) {
	resource, err := passkeyutils.FindPasskeyByCredentialIDAndUserID(credentialID, userID)
	if err != nil {
		sharedutils.HandleValidationError(w, err)
		return
	}
	passkeyName := resource.GetName()

	// Check if this is the last passkey (unless forceLastDelete is true)
	if !forceLastDelete {
		if !passkeyutils.ValidateLastPasskeyDeletion(w, userID) {
			return
		}
	}

	lock := concurrency.GetLock(passkeyName)
	lock.Lock()
	defer lock.Unlock()

	deleteResult := api.DeleteCustomResourceByName(passkeyName, authmetadata.UserPasskeyMetadata)
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
