package protection

import (
	"context"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	discoveryauthz "github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
)

func Decide(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	var req planseps.DecideProtectionPlanRequest
	if !decodeBody(w, r, &req) {
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

	ctx, cancel := context.WithTimeout(detached(r), constants.ProtectionPlanDeployTimeout)
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
