package plans

import "github.com/telark/rest/base"

const (
	// Exporter endpoints — CRUD for the ProtectionPlan CRD.
	CreateProtectionPlan     base.Endpoint = "plans/protection/create"
	ListProtectionPlans      base.Endpoint = "plans/protection/get"
	GetProtectionPlanByID    base.Endpoint = "plans/protection/{id}/get"
	PatchProtectionPlanByID  base.Endpoint = "plans/protection/{id}/patch"
	DeleteProtectionPlanByID base.Endpoint = "plans/protection/{id}/delete"

	// Discovery endpoints — orchestration and template catalog.
	PrepareProtectionPlan       base.Endpoint = "plans/protection/prepare"
	GetProtectionPlanTemplates  base.Endpoint = "plans/protection/templates"
	CancelProtectionPlan        base.Endpoint = "plans/protection/{id}/cancel"
	ClearProtectionPlan         base.Endpoint = "plans/protection/{id}/clear"
	GetProtectionPlanStatus     base.Endpoint = "plans/protection/{id}/status"
	GetProtectionPlanViolations base.Endpoint = "plans/protection/{id}/violations"
	DuplicateProtectionPlan     base.Endpoint = "plans/protection/{id}/duplicate"
	ReactivateProtectionPlan    base.Endpoint = "plans/protection/{id}/reactivate"
	UpdateProtectionPlan        base.Endpoint = "plans/protection/{id}/update"
)
