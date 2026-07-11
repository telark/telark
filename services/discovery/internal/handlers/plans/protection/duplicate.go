package protection

import (
	"context"
	"encoding/json"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/core/plans/protection"
	"github.com/telark/discovery/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Duplicate(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.DuplicateProtectionPlanRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	plan, err := globalService.Duplicate(ctx, userID, planID, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, dataerrors.Error(err.Error()), err)
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
