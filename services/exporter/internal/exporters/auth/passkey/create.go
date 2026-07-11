package passkey

import (
	"net/http"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/internal/exporters/generics"
	passkeyutils "github.com/telark/exporter/internal/utils/auth/passkey"
	"github.com/telark/exporter/internal/utils/concurrency"
	userutils "github.com/telark/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

	generics.GenericCreateCustomResource(w, authmetadata.UserPasskeyMetadata, passkeyName, spec)
}
