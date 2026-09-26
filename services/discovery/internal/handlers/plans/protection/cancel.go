package protection

import (
	"context"
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Cancel(w http.ResponseWriter, r *http.Request) {
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

	var req planseps.CancelProtectionPlanRequest
	if !decodeOptionalBody(w, r, &req) {
		return
	}
	reason := constants.EmptyString
	if req.Reason != nil {
		reason = *req.Reason
	}

	ctx, cancel := context.WithTimeout(detached(r), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	release, ok := lockPlanDecision(ctx, w, planID)
	if !ok {
		return
	}
	defer release()

	plan, err := svc.Cancel(ctx, userID, planID, reason)
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
