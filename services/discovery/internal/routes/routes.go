package routes

import (
	"github.com/telark/discovery/internal/constants"
	resourceslist "github.com/telark/discovery/internal/handlers/analyze/resources"
	workloadslist "github.com/telark/discovery/internal/handlers/analyze/workloads"
	insightshandler "github.com/telark/discovery/internal/handlers/insights"
	namespacehandler "github.com/telark/discovery/internal/handlers/namespaces"
	protectionplanhandler "github.com/telark/discovery/internal/handlers/plans/protection"
	applicationhandler "github.com/telark/discovery/internal/handlers/resources/applications"
	statushandler "github.com/telark/discovery/internal/handlers/status"
	"github.com/telark/rest/base"
	analyzeps "github.com/telark/rest/endpoints/analyze"
	insightseps "github.com/telark/rest/endpoints/insights"
	planseps "github.com/telark/rest/endpoints/plans"
	applicationeps "github.com/telark/rest/endpoints/resources/applications"
	"github.com/telark/rest/router"
)

var Routes = []router.Route{
	// Analyze routes
	router.CreateRoute(base.Get, analyzeps.GetAllWorkloadsByNamespace, workloadslist.ListNamespaceWorkloads),
	router.CreateRoute(base.Get, analyzeps.GetAllResourcesByNamespace, resourceslist.ListNamespaceResources),

	// Insights read (windowed to the caller's visible apps)
	router.CreateRoute(base.Get, insightseps.Applications, insightshandler.GetApplicationsInsights),

	// Application routes
	router.CreateRoute(base.Post, applicationeps.TriggerRollback, applicationhandler.TriggerRollback),
	router.CreateRoute(base.Post, applicationeps.AbortRollback, applicationhandler.AbortRollback),
	router.CreateRoute(base.Post, applicationeps.SyncApplication, applicationhandler.SyncApplication),
	router.CreateRoute(base.Post, applicationeps.ResetApplication, applicationhandler.ResetApplication),
	router.CreateRoute(base.Get, applicationeps.DiscoveryStatus, applicationhandler.DiscoveryStatus),

	// Namespace routes
	router.CreateRoute(base.Get, analyzeps.GetAllNamespaces, namespacehandler.GetNamespaces),

	// Protection plan routes
	router.CreateRoute(base.Get, planseps.GetProtectionPlanTemplates, protectionplanhandler.GetTemplates),
	router.CreateRoute(base.Post, planseps.PrepareProtectionPlan, protectionplanhandler.Prepare),
	router.CreateRoute(base.Post, planseps.CancelProtectionPlan, protectionplanhandler.Cancel),
	router.CreateRoute(base.Delete, planseps.ClearProtectionPlan, protectionplanhandler.Clear),
	router.CreateRoute(base.Get, planseps.GetProtectionPlanStatus, protectionplanhandler.Status),
	router.CreateRoute(base.Get, planseps.GetProtectionPlanViolations, protectionplanhandler.Violations),
	router.CreateRoute(base.Post, planseps.DuplicateProtectionPlan, protectionplanhandler.Duplicate),
	router.CreateRoute(base.Post, planseps.ReactivateProtectionPlan, protectionplanhandler.Reactivate),
	router.CreateRoute(base.Post, planseps.UpdateProtectionPlan, protectionplanhandler.Update),

	// Status routes
	router.CreateRoute(base.Get, constants.StatusReadinessEp, statushandler.Readiness),
	router.CreateRoute(base.Get, constants.StatusLivenessEp, statushandler.Liveness),
}
