package health

import (
	"github.com/telark/data/plans"
	"github.com/telark/discovery/constants"
	planseps "github.com/telark/rest/endpoints/plans"
)

// ToPatch turns a Result into the exporter PATCH shape used by the health controllers and
// the on-demand health endpoint. Detail rows are only attached when present so we don't blow
// away an existing detail list with an empty array on early-failure paths.
func ToPatch(result Result, now string) planseps.PatchProtectionPlanRequest {
	health := result.Health
	checked := now
	patch := planseps.PatchProtectionPlanRequest{
		Health:          &health,
		HealthCheckedAt: &checked,
	}
	if len(result.Detail) > constants.DefaultInitValue {
		patch.HealthDetail = ToDetailRequests(result.Detail)
	}
	return patch
}

func ToDetailRequests(items []plans.ProtectionPlanHealthDetail) []planseps.HealthDetailRequest {
	out := make([]planseps.HealthDetailRequest, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, planseps.HealthDetailRequest{
			PolicyName:    item.PolicyName,
			Namespace:     item.Namespace,
			Present:       item.Present,
			Ready:         item.Ready,
			FailureAction: item.FailureAction,
		})
	}
	return out
}
