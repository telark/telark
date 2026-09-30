package authz

import (
	"context"

	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	applicationeps "github.com/telark/telark/internal/rest/endpoints/applications"
	clustereps "github.com/telark/telark/internal/rest/endpoints/cluster"
	insightseps "github.com/telark/telark/internal/rest/endpoints/insights"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addCluster(requirements)
	addApplications(requirements)
	addInsights(requirements)
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
func addCluster(r map[string]authz.Requirement) {
	r[router.Key(base.Get, clustereps.GetAllWorkloadsByNamespace)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, clustereps.GetAllResourcesByNamespace)] = authz.Read(roledata.ScopeApplications)
	// Either of two scopes reaches it, which a Requirement cannot express: the handler checks NamespacesAllowed.
	r[router.Key(base.Get, clustereps.GetAllNamespaces)] = authz.Authenticated
}

// Rollback, sync and reset mutate live workloads through this service's
// cluster-wide write access, so they are the most consequential routes here.
func addApplications(r map[string]authz.Requirement) {
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

func addInsights(r map[string]authz.Requirement) {
	r[router.Key(base.Get, insightseps.Applications)] = authz.Read(roledata.ScopeInsights)
	r[router.Key(base.Get, insightseps.List)] = authz.Read(roledata.ScopeInsights)
}

func addPlans(r map[string]authz.Requirement) {
	view := authz.Denyable(authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans)
	r[router.Key(base.Get, planseps.GetProtectionPlanTemplates)] = view
	r[router.Key(base.Get, planseps.GetProtectionPlanStatus)] = view
	r[router.Key(base.Get, planseps.GetProtectionPlanViolations)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanViolations)
	r[router.Key(base.Post, planseps.PrepareProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan)
	r[router.Key(base.Post, planseps.CancelProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCancelProtectionPlan)
	r[router.Key(base.Post, planseps.DuplicateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionDuplicateProtectionPlan)
	r[router.Key(base.Post, planseps.ReactivateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionReactivateProtectionPlan)
	// Every field, including spec options added later, is edited through this one route.
	r[router.Key(base.Post, planseps.ReviseProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlan)
	// Approve and reject share the route; the handler applies each decision's own rule.
	r[router.Key(base.Post, planseps.DecideProtectionPlan)] = authz.Own(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, planseps.GenerateProtectionPlanReport)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionGenerateProtectionPlanReport)
	r[router.Key(base.Delete, planseps.ClearProtectionPlan)] = authz.Denyable(
		authz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlan)
}

func ApprovePlanRequirement() authz.Requirement {
	return authz.Denyable(authz.Own(roledata.ScopeProtectionPlans), roledata.ActionApproveProtectionPlan)
}

func RejectPlanRequirement() authz.Requirement {
	return authz.Denyable(authz.Own(roledata.ScopeProtectionPlans), roledata.ActionRejectProtectionPlan)
}

// Internal peers pass, as they do in the route middleware.
func RequestAllows(ctx context.Context, requirement authz.Requirement) bool {
	identity, ok := authz.FromContext(ctx)
	return ok && (identity.Internal || authz.Allows(identity, requirement))
}

// The applications and insights pages both filter by namespace.
func NamespacesAllowed(ctx context.Context) bool {
	return RequestAllows(ctx, authz.Read(roledata.ScopeApplications)) ||
		RequestAllows(ctx, authz.Read(roledata.ScopeInsights))
}
