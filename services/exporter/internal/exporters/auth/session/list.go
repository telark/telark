package session

import (
	"net/http"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sessionutils "github.com/telark/telark/services/exporter/internal/utils/auth/session"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func ListSessionsByUser(w http.ResponseWriter, userID string) {
	if !userutils.ValidateUserOrRespond(w, userID) {
		return
	}

	sessions, err := sessionutils.FindSessionsByUserID(userID)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return
	}

	if len(sessions) == constants.DefaultInitValue {
		responseutils.LogAndSendResponse(
			w,
			http.StatusOK,
			response.OperationSuccess,
			string(messages.SuccessListRes),
			map[string]any{constants.FieldItems: []any{}},
			nil,
		)
		return
	}

	filteredItems := make([]any, constants.DefaultInitValue, len(sessions))
	for i := range sessions {
		filtered, err := sharedutils.FilterData(&sessions[i])
		if err != nil {
			continue
		}
		filteredItems = append(filteredItems, filtered)
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		map[string]any{constants.FieldItems: filteredItems},
		nil,
	)
}
