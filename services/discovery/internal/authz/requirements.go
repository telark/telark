package authz

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/base"
	analyzeps "github.com/telark/rest/endpoints/analyze"
	insightseps "github.com/telark/rest/endpoints/insights"
	planseps "github.com/telark/rest/endpoints/plans"
	applicationeps "github.com/telark/rest/endpoints/resources/applications"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/authz"
)

func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addAnalyze(requirements)
	addApplications(requirements)
	addPlans(requirements)

	return requirements
}

// Called by kubelet, which holds no session.
func addStatus(r map[string]authz.Requirement) {
	r[router.Key(base.Get, constants.StatusLivenessEp)] = authz.Public
	r[router.Key(base.Get, constants.StatusReadinessEp)] = authz.Public
}

// Reading cluster state exposes what is deployed and how, so it follows the
// applications scope rather than being open to any authenticated user.
func addAnalyze(r map[string]authz.Requirement) {
	r[router.Key(base.Get, analyzeps.GetAllWorkloadsByNamespace)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, analyzeps.GetAllResourcesByNamespace)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, analyzeps.GetAllNamespaces)] = authz.Read(roledata.ScopeApplications)
}

// Rollback, sync and reset mutate live workloads through this service's
// cluster-wide write access, so they are the most consequential routes here.
func addApplications(r map[string]authz.Requirement) {
	r[router.Key(base.Get, insightseps.Applications)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationeps.DiscoveryStatus)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Post, applicationeps.TriggerRollback)] = authz.Denyable(
		authz.Write(roledata.ScopeApplications), roledata.ActionRollbackApplication)
	r[router.Key(base.Post, applicationeps.AbortRollback)] = authz.Denyable(
		authz.Write(roledata.ScopeApplications), roledata.ActionRollbackApplication)
	r[router.Key(base.Post, applicationeps.SyncApplication)] = authz.Denyable(
		authz.Write(roledata.ScopeApplications), roledata.ActionForceApplicationSync)
	r[router.Key(base.Post, applicationeps.ResetApplication)] = authz.Denyable(
		authz.Own(roledata.ScopeApplications), roledata.ActionDeleteApplication)
}

func addPlans(r map[string]authz.Requirement) {
	r[router.Key(base.Get, planseps.GetProtectionPlanTemplates)] = authz.Read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanStatus)] = authz.Read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanViolations)] = authz.Read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.PrepareProtectionPlan)] = authz.Write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.CancelProtectionPlan)] = authz.Write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.DuplicateProtectionPlan)] = authz.Write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.ReactivateProtectionPlan)] = authz.Write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.UpdateProtectionPlan)] = authz.Write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Delete, planseps.ClearProtectionPlan)] = authz.Own(roledata.ScopeProtectionPlans)
}
