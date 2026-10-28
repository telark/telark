package protection

import (
	"context"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	requestutils "github.com/telark/telark/internal/rest/utils/request"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
)

func Clear(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	if _, ok = requireUser(w, r); !ok {
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}
	keepReports, err := requestutils.OptionalBoolQuery(r, planseps.QueryKeepReports)
	if err != nil {
		respondError(w, http.StatusBadRequest, dataerrors.Error(err.Error()), err)
		return
	}

	ctx, cancel := context.WithTimeout(detached(r), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	release, ok := lockPlanDecision(ctx, w, planID)
	if !ok {
		return
	}
	defer release()

	if err := svc.Clear(ctx, planID, keepReports); err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessDeleteRes, planID),
		nil,
		nil,
	)
}
