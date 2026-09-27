package session

import (
	"net/http"

	authmetadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/exporters/generics"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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
