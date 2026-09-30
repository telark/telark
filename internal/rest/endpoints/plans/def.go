package plans

import "github.com/telark/telark/internal/rest/base"

const (
	// Served by the exporter
	CreateProtectionPlan     base.Endpoint = "protectionplans"
	ListProtectionPlans      base.Endpoint = "protectionplans"
	GetProtectionPlanByID    base.Endpoint = "protectionplans/{id}"
	PatchProtectionPlanByID  base.Endpoint = "protectionplans/{id}"
	DeleteProtectionPlanByID base.Endpoint = "protectionplans/{id}"

	// Served by discovery
	PrepareProtectionPlan        base.Endpoint = "protectionplans/prepare"
	GetProtectionPlanTemplates   base.Endpoint = "policytemplates"
	CancelProtectionPlan         base.Endpoint = "protectionplans/{id}/cancel"
	ClearProtectionPlan          base.Endpoint = "protectionplans/{id}/clear"
	GetProtectionPlanStatus      base.Endpoint = "protectionplans/{id}/status"
	GetProtectionPlanViolations  base.Endpoint = "protectionplans/{id}/violations"
	DuplicateProtectionPlan      base.Endpoint = "protectionplans/{id}/duplicate"
	ReactivateProtectionPlan     base.Endpoint = "protectionplans/{id}/reactivate"
	ReviseProtectionPlan         base.Endpoint = "protectionplans/{id}/revise"
	GenerateProtectionPlanReport base.Endpoint = "protectionplans/{id}/reports"
	DecideProtectionPlan         base.Endpoint = "protectionplans/{id}/decision"
)
