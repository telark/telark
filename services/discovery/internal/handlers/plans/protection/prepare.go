package protection

import (
	"context"
	"errors"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func Prepare(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	var req planseps.PrepareProtectionPlanRequest
	if !decodeBody(w, r, &req) {
		return
	}

	ctx, cancel := context.WithTimeout(detached(r), constants.ProtectionPlanDeployTimeout)
	defer cancel()

	plan, err := svc.Prepare(ctx, userID, &req)
	if err != nil {
		respondDomainError(w, err)
		return
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessCreateRes, plan.ID),
		plan,
		nil,
	)
}

func respondError(w http.ResponseWriter, status int, errCode dataerrors.Error, err error) {
	// A refusal is the caller's answer, not a failure: only a 5xx is logged.
	if status < http.StatusInternalServerError {
		responseutils.SendResponse(w, status, response.OperationError, string(errCode), nil)
		return
	}
	if err == nil {
		err = errors.New(string(errCode))
	}
	responseutils.LogAndSendResponse(w, status, response.OperationError, string(errCode), nil, err)
}
