package protection

import (
	"context"
	"encoding/json"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Cancel(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.CancelProtectionPlanRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	reason := constants.EmptyString
	if req.Reason != nil {
		reason = *req.Reason
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	plan, err := globalService.Cancel(ctx, userID, planID, reason)
	if err != nil {
		respondError(w, http.StatusBadRequest, dataerrors.Error(err.Error()), err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessUpdateRes),
		plan,
		nil,
	)
}
