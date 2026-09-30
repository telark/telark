package routes

import (
	"github.com/telark/telark/internal/rest/base"
	applicationeps "github.com/telark/telark/internal/rest/endpoints/applications"
	clustereps "github.com/telark/telark/internal/rest/endpoints/cluster"
	insightseps "github.com/telark/telark/internal/rest/endpoints/insights"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/discovery/internal/constants"
	resourceslist "github.com/telark/telark/services/discovery/internal/handlers/analyze/resources"
	workloadslist "github.com/telark/telark/services/discovery/internal/handlers/analyze/workloads"
	insightshandler "github.com/telark/telark/services/discovery/internal/handlers/insights"
	namespacehandler "github.com/telark/telark/services/discovery/internal/handlers/namespaces"
	protectionplanhandler "github.com/telark/telark/services/discovery/internal/handlers/plans/protection"
	applicationhandler "github.com/telark/telark/services/discovery/internal/handlers/resources/applications"
	statushandler "github.com/telark/telark/services/discovery/internal/handlers/status"
)

var Routes = []router.Route{
	// Cluster read routes
	router.CreateRoute(base.Get, clustereps.GetAllWorkloadsByNamespace, workloadslist.ListNamespaceWorkloads),
	router.CreateRoute(base.Get, clustereps.GetAllResourcesByNamespace, resourceslist.ListNamespaceResources),

	// Insights read (windowed to the caller's visible apps)
	router.CreateRoute(base.Get, insightseps.Applications, insightshandler.GetApplicationsInsights),
	// Cluster-wide insights list (served from the per-replica row index)
	router.CreateRoute(base.Get, insightseps.List, insightshandler.ListInsights),

	// Application routes
	router.CreateRoute(base.Post, applicationeps.TriggerRollback, applicationhandler.TriggerRollback),
	router.CreateRoute(base.Post, applicationeps.AbortRollback, applicationhandler.AbortRollback),
	router.CreateRoute(base.Post, applicationeps.SyncApplication, applicationhandler.SyncApplication),
	router.CreateRoute(base.Post, applicationeps.ResetApplication, applicationhandler.ResetApplication),
	router.CreateRoute(base.Get, applicationeps.DiscoveryStatus, applicationhandler.DiscoveryStatus),

	// Namespace routes
	router.CreateRoute(base.Get, clustereps.GetAllNamespaces, namespacehandler.GetNamespaces),

	// Protection plan routes
	router.CreateRoute(base.Get, planseps.GetProtectionPlanTemplates, protectionplanhandler.GetTemplates),
	router.CreateRoute(base.Post, planseps.PrepareProtectionPlan, protectionplanhandler.Prepare),
	router.CreateRoute(base.Post, planseps.CancelProtectionPlan, protectionplanhandler.Cancel),
	router.CreateRoute(base.Delete, planseps.ClearProtectionPlan, protectionplanhandler.Clear),
	router.CreateRoute(base.Get, planseps.GetProtectionPlanStatus, protectionplanhandler.Status),
	router.CreateRoute(base.Get, planseps.GetProtectionPlanViolations, protectionplanhandler.Violations),
	router.CreateRoute(base.Post, planseps.DuplicateProtectionPlan, protectionplanhandler.Duplicate),
	router.CreateRoute(base.Post, planseps.ReactivateProtectionPlan, protectionplanhandler.Reactivate),
	router.CreateRoute(base.Post, planseps.ReviseProtectionPlan, protectionplanhandler.Update),
	router.CreateRoute(base.Post, planseps.DecideProtectionPlan, protectionplanhandler.Decide),
	router.CreateRoute(base.Post, planseps.GenerateProtectionPlanReport, protectionplanhandler.GenerateReport),

	// Status routes
	router.CreateRoute(base.Get, constants.StatusReadinessEp, statushandler.Readiness),
	router.CreateRoute(base.Get, constants.StatusLivenessEp, statushandler.Liveness),
}
