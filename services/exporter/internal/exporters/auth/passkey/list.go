package passkey

import (
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/exporter/internal/constants"
	passkeyutils "github.com/telark/exporter/internal/utils/auth/passkey"
	userutils "github.com/telark/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func ListPasskeysByUser(w http.ResponseWriter, userID string) {
	if !userutils.ValidateUserOrRespond(w, userID) {
		return
	}

	passkeys, err := passkeyutils.FindPasskeysByUserID(userID)
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

	if len(passkeys) == constants.DefaultInitValue {
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

	filteredItems := make([]any, constants.DefaultInitValue, len(passkeys))
	for i := range passkeys {
		filtered, err := sharedutils.FilterData(&passkeys[i])
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
