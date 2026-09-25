package protection

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	discoveryauthz "github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Decide(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.DecideProtectionPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		msg := fmt.Sprintf(string(protection.ErrRequestBody), err)
		respondError(w, http.StatusBadRequest, dataerrors.Error(msg), err)
		return
	}

	requirement := discoveryauthz.ApprovePlanRequirement()
	if req.Decision == protection.DecisionRejected {
		requirement = discoveryauthz.RejectPlanRequirement()
	}
	if !discoveryauthz.RequestAllows(r.Context(), requirement) {
		respondError(w, http.StatusForbidden, dataerrors.ErrAuthzForbidden, nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanDeployTimeout)
	defer cancel()

	release, ok := lockPlanDecision(ctx, w, planID)
	if !ok {
		return
	}
	defer release()

	plan, err := svc.Decide(ctx, userID, planID, req)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessUpdateRes, planID),
		plan,
		nil,
	)
}
