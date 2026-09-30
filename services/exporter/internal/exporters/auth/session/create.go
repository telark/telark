package session

import (
	"net/http"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/exporters/generics"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	"github.com/telark/telark/services/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func CreateSessionByUser(w http.ResponseWriter, body map[string]any, userID string) {
	go sessionutils.PurgeExpiredSessionsForUser(userID)

	session, sessionName, err := sessionutils.ExtractSessionSpec(body, userID)
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

	spec, err := sharedutils.StructToSpecMap(session)
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

	lock := concurrency.GetLock(sessionName)
	lock.Lock()
	defer lock.Unlock()

	generics.GenericCreateCustomResource(w, authmetadata.SessionMetadata, sessionName, spec)
}
