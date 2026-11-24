package authrequests

import (
	"errors"
	"net/http"

	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authrequests"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

var refusals = map[error]int{
	authrequests.ErrPhase:    http.StatusBadRequest,
	authrequests.ErrNotFound: http.StatusNotFound,
}

func GetPending() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		pending, err := authrequests.Pending()
		if err != nil {
			responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
			return
		}
		responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, string(constants.MsgAuthRequestsListed),
			map[string]any{constants.FieldItems: pending})
	}
}

func PatchStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		body, err := sharedutils.GetSpec(w, r)
		if err != nil {
			return
		}
		status, err := sharedutils.ExtractStructFromBody[authrequest.Status](body)
		if err != nil {
			responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil)
			return
		}
		err = authrequests.SetStatus(id, *status)
		if err == nil {
			responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, string(constants.MsgAuthRequestStatusUpdated), nil)
			return
		}
		for refusal, code := range refusals {
			if errors.Is(err, refusal) {
				responseutils.SendResponse(w, code, response.OperationError, err.Error(), nil)
				return
			}
		}
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
	}
}
