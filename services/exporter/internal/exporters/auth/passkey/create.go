package passkey

import (
	"net/http"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	passkeyutils "github.com/telark/telark/services/exporter/internal/utils/auth/passkey"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreatePasskeyByUser(w http.ResponseWriter, body map[string]any, userID string) {
	if !userutils.ValidateUserOrRespond(w, userID) {
		return
	}

	passkey, passkeyName, err := passkeyutils.ExtractPasskeySpec(body, userID)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			nil,
		)
		return
	}

	spec, err := sharedutils.StructToSpecMap(passkey)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			nil,
		)
		return
	}

	lock := concurrency.GetLock(passkeyName)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericCreateCustomResource(w, authmetadata.PasskeyMetadata, passkeyName, spec)
}
