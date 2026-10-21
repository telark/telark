package bootstrap

import (
	"errors"
	"net/http"

	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

var refusals = map[error]int{
	bootstrap.ErrEmail:       http.StatusBadRequest,
	bootstrap.ErrReason:      http.StatusBadRequest,
	bootstrap.ErrNoRequest:   http.StatusNotFound,
	bootstrap.ErrNotPinned:   http.StatusConflict,
	bootstrap.ErrNotPending:  http.StatusConflict,
	bootstrap.ErrExpired:     http.StatusConflict,
	bootstrap.ErrNotEligible: http.StatusConflict,
	bootstrap.ErrEnrolled:    http.StatusConflict,
	bootstrap.ErrReissue:     http.StatusConflict,
	bootstrap.ErrContended:   http.StatusConflict,
	bootstrap.ErrNoTarget:    http.StatusConflict,
}

func GetStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, notifyAdmins, err := bootstrap.View()
		if err != nil {
			responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
			return
		}
		// A replacement's target is or may become the bootstrap account.
		masked := authz.MasksIdentity(r, status.UserID, true)
		if masked {
			status.Email = constants.EmptyString
			for i := range status.Requests {
				status.Requests[i].TargetEmail = constants.EmptyString
			}
		}
		// With alerts off, the record is the bootstrap account's alone, like the notices.
		if masked && !notifyAdmins {
			status.Requests = nil
		}
		responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, string(constants.MsgBootstrapStatusRead),
			eps.StatusResult{BootstrapStatus: *status, NotifyAdmins: notifyAdmins})
	}
}

func OpenRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := decode[eps.OpenBody](w, r)
		if !ok {
			return
		}
		outcome, err := bootstrap.Open(body.Email)
		respond(w, outcome, err)
	}
}

func ApproveRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		outcome, err := bootstrap.Approve(id)
		respond(w, outcome, err)
	}
}

func CancelRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		body, ok := decode[eps.CancelBody](w, r)
		if !ok {
			return
		}
		outcome, err := bootstrap.Cancel(id, body.Reason)
		respond(w, outcome, err)
	}
}

func CompleteRequest(optimizer *performance.Optimizer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		body, ok := decode[eps.CompleteBody](w, r)
		if !ok {
			return
		}
		outcome, err := bootstrap.Complete(optimizer, id, body.Revoke)
		respond(w, outcome, err)
	}
}

func decode[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	body, err := sharedutils.GetSpec(w, r)
	if err != nil {
		return nil, false
	}
	typed, err := sharedutils.ExtractStructFromBody[T](body)
	if err != nil {
		responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil)
		return nil, false
	}
	return typed, true
}

// A completion whose revocation failed after the status was written still reports the completion.
func respond(w http.ResponseWriter, outcome *bootstrap.Outcome, err error) {
	for _, request := range outcome.Changed {
		title, message, masked, severity := notice(request)
		notify(outcome.Pinned, request, title, message, masked, severity)
	}
	if outcome.Reissued {
		notify(outcome.Pinned, outcome.Result.Request, constants.NotifBootstrapReissuedTitle,
			constants.NotifBootstrapReissuedMessage, constants.NotifBootstrapReissuedMessage, notiftypes.SeverityWarning)
	}
	if err == nil {
		responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, string(constants.MsgBootstrapRequestUpdated), outcome.Result)
		return
	}
	for refusal, status := range refusals {
		if errors.Is(err, refusal) {
			responseutils.SendResponse(w, status, response.OperationError, err.Error(), outcome.Result)
			return
		}
	}
	responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError, err.Error(), nil, err)
}
