package protection

import (
	"net/http"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Status(w http.ResponseWriter, r *http.Request) {
	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	plan, result, err := globalService.HealthCheck(r.Context(), planID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, dataerrors.Error(err.Error()), err)
		return
	}

	resp := planseps.ProtectionPlanStatusResponse{
		PlanID:   plan.ID,
		Phase:    plan.Phase,
		Health:   result.Health,
		Policies: result.Policies,
		Drift: planseps.ProtectionPlanDrift{
			Missing:    result.Missing,
			Unexpected: result.Unexpected,
		},
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessGetRes),
		resp,
		nil,
	)
}
