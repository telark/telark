package session

import (
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/exporter/constants"
	sessionutils "github.com/telark/exporter/utils/auth/session"
	userutils "github.com/telark/exporter/utils/resources/user"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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
