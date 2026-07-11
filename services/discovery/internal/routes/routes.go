package routes

import (
	"github.com/telark/discovery/constants"
	resourceslist "github.com/telark/discovery/handlers/analyze/resources"
	workloadslist "github.com/telark/discovery/handlers/analyze/workloads"
	namespacehandler "github.com/telark/discovery/handlers/namespaces"
	protectionplanhandler "github.com/telark/discovery/handlers/plans/protection"
	applicationhandler "github.com/telark/discovery/handlers/resources/applications"
	statushandler "github.com/telark/discovery/handlers/status"
	"github.com/telark/rest/base"
	analyzeps "github.com/telark/rest/endpoints/analyze"
	planseps "github.com/telark/rest/endpoints/plans"
	applicationeps "github.com/telark/rest/endpoints/resources/applications"
	"github.com/telark/rest/router"
)

var Routes = []router.Route{
	// Analyze routes
	router.CreateRoute(base.Get, analyzeps.GetAllWorkloadsByNamespace, workloadslist.ListNamespaceWorkloads),
	router.CreateRoute(base.Get, analyzeps.GetAllResourcesByNamespace, resourceslist.ListNamespaceResources),

	// Application routes
	router.CreateRoute(base.Get, applicationeps.EnrichApplications, applicationhandler.EnrichApplications),
	router.CreateRoute(base.Post, applicationeps.TriggerRollback, applicationhandler.TriggerRollback),
	router.CreateRoute(base.Post, applicationeps.AbortRollback, applicationhandler.AbortRollback),
	router.CreateRoute(base.Post, applicationeps.SyncApplication, applicationhandler.SyncApplication),
	router.CreateRoute(base.Delete, applicationeps.CleanupApplication, applicationhandler.CleanupApplicationData),

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
	router.CreateRoute(base.Get, constants.StatusReadinessEp, statushandler.ProbeHandler),
	router.CreateRoute(base.Get, constants.StatusLivenessEp, statushandler.ProbeHandler),
}
