package protection

import (
	"context"
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanDeployTimeout)
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
	if err == nil {
		err = errors.New(string(errCode))
	}
	responseutils.LogAndSendResponse(w, status, response.OperationError, string(errCode), nil, err)
}
