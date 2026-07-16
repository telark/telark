package authz

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/base"
	analyzeps "github.com/telark/rest/endpoints/analyze"
	planseps "github.com/telark/rest/endpoints/plans"
	applicationeps "github.com/telark/rest/endpoints/resources/applications"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/authz"
)

var public = authz.Requirement{Access: authz.AccessPublic}

func read(scope string) authz.Requirement {
	return authz.Requirement{Scope: scope, MinLevel: roledata.PermissionLevelReadOnly}
}

func write(scope string) authz.Requirement {
	return authz.Requirement{Scope: scope, MinLevel: roledata.PermissionLevelContributor}
}

func remove(scope string) authz.Requirement {
	return authz.Requirement{Scope: scope, MinLevel: roledata.PermissionLevelOwner}
}

// denyable marks a route a role can withhold on its own, using the action keys
// the dashboard offers when editing that role.
func denyable(requirement authz.Requirement, action string) authz.Requirement {
	requirement.Rule = authz.RuleKey(requirement.Scope, action)
	return requirement
}

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
	r[router.Key(base.Get, constants.StatusLivenessEp)] = public
	r[router.Key(base.Get, constants.StatusReadinessEp)] = public
}

// Reading cluster state exposes what is deployed and how, so it follows the
// applications scope rather than being open to any authenticated user.
func addAnalyze(r map[string]authz.Requirement) {
	r[router.Key(base.Get, analyzeps.GetAllWorkloadsByNamespace)] = read(roledata.ScopeApplications)
	r[router.Key(base.Get, analyzeps.GetAllResourcesByNamespace)] = read(roledata.ScopeApplications)
	r[router.Key(base.Get, analyzeps.GetAllNamespaces)] = read(roledata.ScopeApplications)
}

// Rollback, sync and cleanup mutate live workloads through this service's
// cluster-wide write access, so they are the most consequential routes here.
func addApplications(r map[string]authz.Requirement) {
	r[router.Key(base.Get, applicationeps.EnrichApplications)] = read(roledata.ScopeApplications)
	r[router.Key(base.Post, applicationeps.TriggerRollback)] = denyable(
		write(roledata.ScopeApplications), roledata.ActionRollbackApplication)
	r[router.Key(base.Post, applicationeps.AbortRollback)] = denyable(
		write(roledata.ScopeApplications), roledata.ActionRollbackApplication)
	r[router.Key(base.Post, applicationeps.SyncApplication)] = denyable(
		write(roledata.ScopeApplications), roledata.ActionForceApplicationSync)
	r[router.Key(base.Delete, applicationeps.CleanupApplication)] = denyable(
		remove(roledata.ScopeApplications), roledata.ActionDeleteApplication)
}

func addPlans(r map[string]authz.Requirement) {
	r[router.Key(base.Get, planseps.GetProtectionPlanTemplates)] = read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanStatus)] = read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanViolations)] = read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.PrepareProtectionPlan)] = write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.CancelProtectionPlan)] = write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.DuplicateProtectionPlan)] = write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.ReactivateProtectionPlan)] = write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.UpdateProtectionPlan)] = write(roledata.ScopeProtectionPlans)
	r[router.Key(base.Delete, planseps.ClearProtectionPlan)] = remove(roledata.ScopeProtectionPlans)
}
