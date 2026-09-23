package protection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Prepare(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return
	}

	var req planseps.PrepareProtectionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf(string(protection.ErrRequestBody), err)
		respondError(w, http.StatusBadRequest, dataerrors.Error(msg), err)
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
		string(messages.SuccessCreateRes),
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
