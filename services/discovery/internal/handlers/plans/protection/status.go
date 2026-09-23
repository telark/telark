package protection

import (
	"net/http"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/shared"
	planseps "github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func Status(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	plan, result, err := svc.HealthCheck(r.Context(), planID)
	if err != nil {
		respondDomainError(w, err)
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
