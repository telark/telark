package protection

import (
	"context"
	"net/http"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/data/text"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
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
	reason, err = text.Check(reason, text.Rule{Field: constants.FieldReason, Max: constants.ReasonMaxLen})
	if err != nil {
		respondDomainError(w, validation.Invalid(err.Error()))
		return
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
