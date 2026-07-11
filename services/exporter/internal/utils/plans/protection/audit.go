package protection

import (
	"time"

	"github.com/telark/data/plans"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/exporter/internal/constants"
)

func ApplyCreateAudit(plan *plans.ProtectionPlan, userID string) {
	now := time.Now().UTC().Format(globalshared.DefaultTimeFormat)
	plan.CreatedAt = now
	plan.CreatedBy = userID
	plan.LastUpdatedAt = now
	plan.LastUpdatedBy = userID

	if plan.Phase == constants.PhaseActive {
		plan.StartedAt = &now
		plan.StartedBy = &userID
	}
}

func ApplyPatchAudit(body map[string]any, userID string) {
	now := time.Now().UTC().Format(globalshared.DefaultTimeFormat)
	body[constants.FieldLastUpdatedAt] = now
	body[constants.FieldLastUpdatedBy] = userID
	delete(body, constants.FieldCreatedAt)
	delete(body, constants.FieldCreatedBy)
	delete(body, constants.FieldID)
}
