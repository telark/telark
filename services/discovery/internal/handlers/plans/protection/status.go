package protection

import (
	"net/http"

	"github.com/telark/telark/internal/data/messages"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
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
			Mismatched: result.Mismatched,
			Stale:      result.Stale,
			Added:      result.Added,
		},
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessGetRes, planID),
		resp,
		nil,
	)
}
