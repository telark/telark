package routes

import (
	"net/http"

	exporterauthz "github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	passkeyhandler "github.com/telark/exporter/internal/handlers/auth/passkey"
	sessionhandler "github.com/telark/exporter/internal/handlers/auth/session"
	categoryhandler "github.com/telark/exporter/internal/handlers/categories"
	confighandler "github.com/telark/exporter/internal/handlers/config"
	notificationhandler "github.com/telark/exporter/internal/handlers/notifications"
	planshandler "github.com/telark/exporter/internal/handlers/plans/protection"
	reportshandler "github.com/telark/exporter/internal/handlers/reports"
	applicationhandler "github.com/telark/exporter/internal/handlers/resources/application"
	cleanuphandler "github.com/telark/exporter/internal/handlers/resources/cleanup"
	grouphandler "github.com/telark/exporter/internal/handlers/resources/group"
	rolehandler "github.com/telark/exporter/internal/handlers/resources/role"
	userhandler "github.com/telark/exporter/internal/handlers/resources/user"
	snapshothandler "github.com/telark/exporter/internal/handlers/snapshot"
	statushandler "github.com/telark/exporter/internal/handlers/status"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/base"
	roleendpoints "github.com/telark/rest/endpoints/accessroles"
	applicationendpoints "github.com/telark/rest/endpoints/applications"
	authendpoints "github.com/telark/rest/endpoints/auth"
	categoryendpoints "github.com/telark/rest/endpoints/categories"
	cleanupendpoints "github.com/telark/rest/endpoints/cleanup"
	configendpoints "github.com/telark/rest/endpoints/config"
	groupendpoints "github.com/telark/rest/endpoints/groups"
	notificationsendpoints "github.com/telark/rest/endpoints/notifications"
	plansendpoints "github.com/telark/rest/endpoints/plans"
	reportsendpoints "github.com/telark/rest/endpoints/reports"
	snapshotendpoints "github.com/telark/rest/endpoints/snapshots"
	statusendpoints "github.com/telark/rest/endpoints/status"
	userendpoints "github.com/telark/rest/endpoints/users"
	"github.com/telark/rest/router"
)

func userResourceCachedGetRoute(
	optimizer *performance.Optimizer,
	endpoint base.Endpoint,
	handler http.HandlerFunc,
) router.Route {
	return router.CreateRoute(base.Get, endpoint,
		performance.NewCachedListHandlerFunc(
			optimizer,
			handler,
			exporterauthz.RestrictedKey(cache.NewGetCacheKeyFunc(constants.ResourceUser)),
			constants.ResourceUser,
			constants.OpGet,
		))
}

func subjectListCachedRoute(
	optimizer *performance.Optimizer,
	endpoint base.Endpoint,
	handler http.HandlerFunc,
	resourceType string,
	subject cache.SubjectFunc,
) router.Route {
	return router.CreateRoute(base.Get, endpoint,
		performance.NewCachedListHandlerFunc(
			optimizer,
			handler,
			cache.NewSubjectListCacheKeyFunc(optimizer, resourceType, subject),
			resourceType,
			constants.OpList,
		))
}

func InitRoutes(optimizer *performance.Optimizer) []router.Route {
	routes := make([]router.Route, constants.DefaultInitValue, constants.DefaultRoutesCount)
	routes = append(routes, applicationRoutes(optimizer)...)
	routes = append(routes, configRoutes()...)
	routes = append(routes, userRoutes(optimizer)...)
	routes = append(routes, groupRoutes(optimizer)...)
	routes = append(routes, roleRoutes(optimizer)...)
	routes = append(routes, categoryRoutes(optimizer)...)
	routes = append(routes, sessionRoutes(optimizer)...)
	routes = append(routes, passkeyRoutes(optimizer)...)
	routes = append(routes, snapshotRoutes()...)
	routes = append(routes, reportRoutes()...)
	routes = append(routes, notificationRoutes()...)
	routes = append(routes, protectionPlanRoutes()...)
	routes = append(routes, statusRoutes()...)
	routes = append(routes, cleanupRoutes()...)
	return routes
}

func cleanupRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Update, cleanupendpoints.AddFinalizer, cleanuphandler.AddFinalizer),
		router.CreateRoute(base.Delete, cleanupendpoints.RemoveFinalizer, cleanuphandler.RemoveFinalizer),
		router.CreateRoute(base.Get, cleanupendpoints.GetCleanupViewByID, cleanuphandler.GetCleanupViewByID),
		router.CreateRoute(base.Get, cleanupendpoints.ListCleanupViews, cleanuphandler.ListCleanupViews),
	}
}

func statusRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Get, statusendpoints.LivenessCheck, statushandler.Liveness),
		router.CreateRoute(base.Get, statusendpoints.ReadinessCheck, statushandler.Readiness),
	}
}

func protectionPlanRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, plansendpoints.CreateProtectionPlan, planshandler.CreatePlan()),
		router.CreateRoute(base.Get, plansendpoints.ListProtectionPlans, planshandler.ListPlans()),
		router.CreateRoute(base.Get, plansendpoints.GetProtectionPlanByID, planshandler.GetPlanByID()),
		router.CreateRoute(base.Patch, plansendpoints.PatchProtectionPlanByID, planshandler.PatchPlanByID()),
		router.CreateRoute(base.Delete, plansendpoints.DeleteProtectionPlanByID, planshandler.DeletePlanByID()),
	}
}

func reportRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, reportsendpoints.CreatePlanReport, reportshandler.CreatePlanReport()),
		router.CreateRoute(base.Get, reportsendpoints.ListReports, reportshandler.ListReports()),
		router.CreateRoute(base.Get, reportsendpoints.ListPlanReports, reportshandler.ListPlanReports()),
		router.CreateRoute(base.Get, reportsendpoints.DownloadPlanReport, reportshandler.DownloadPlanReport()),
		router.CreateRoute(base.Update, reportsendpoints.PutPlanReportLedger, reportshandler.PutPlanReportLedger()),
		router.CreateRoute(base.Get, reportsendpoints.GetPlanReportLedger, reportshandler.GetPlanReportLedger()),
	}
}

func notificationRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, notificationsendpoints.Emit, notificationhandler.Emit()),
		router.CreateRoute(base.Get, notificationsendpoints.List, notificationhandler.List()),
		router.CreateRoute(base.Post, notificationsendpoints.MarkRead, notificationhandler.MarkRead()),
		router.CreateRoute(base.Post, notificationsendpoints.MarkAllRead, notificationhandler.MarkAllRead()),
		router.CreateRoute(base.Delete, notificationsendpoints.Clear, notificationhandler.Clear()),
	}
}

func configRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Get, configendpoints.GetConfig, confighandler.GetConfig()),
		router.CreateRoute(base.Patch, configendpoints.PatchConfig, confighandler.PatchConfig()),
	}
}

func applicationRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, applicationendpoints.CreateApplication,
			performance.NewDynamicOptimizedHandlerFunc(
				applicationhandler.CreateApplicationResourceWithCacheInvalidation(optimizer),
				constants.ResourceApplication,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetAllApplications,
			performance.NewCachedListHandlerFunc(
				optimizer,
				applicationhandler.ListApplicationResourcesWithCacheInvalidation(),
				cache.NewViewListCacheKeyFunc(optimizer, constants.ResourceApplication),
				constants.ResourceApplication,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, applicationendpoints.GetRollbacks,
			performance.NewDynamicOptimizedHandlerFunc(
				applicationhandler.GetRollbacks(),
				constants.ResourceApplication,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetRollback,
			performance.NewDynamicOptimizedHandlerFunc(
				applicationhandler.GetRollback(),
				constants.ResourceApplication,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetApplicationByName,
			performance.NewCachedListHandlerFunc(
				optimizer,
				applicationhandler.GetApplicationResourceWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceApplication),
				constants.ResourceApplication,
				constants.OpGet,
			)),
		router.CreateRoute(base.Patch, applicationendpoints.PatchApplicationByName,
			performance.NewDynamicOptimizedHandlerFunc(
				applicationhandler.PatchApplicationResourceWithCacheInvalidation(optimizer),
				constants.ResourceApplication,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, applicationendpoints.DeleteApplicationByName,
			performance.NewDynamicOptimizedHandlerFunc(
				applicationhandler.DeleteApplicationResourceWithCacheInvalidation(optimizer),
				constants.ResourceApplication,
				constants.OpDelete,
			),
		),
	}
}

func userRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, userendpoints.CreateUser,
			performance.NewDynamicOptimizedHandlerFunc(
				userhandler.CreateUserResourceWithCacheInvalidation(optimizer),
				constants.ResourceUser,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, userendpoints.GetAllUsers,
			performance.NewCachedListHandlerFunc(
				optimizer,
				userhandler.ListUserResourcesWithCacheInvalidation(),
				exporterauthz.RestrictedKey(cache.NewListCacheKeyFunc(optimizer, constants.ResourceUser)),
				constants.ResourceUser,
				constants.OpList,
			)),
		userResourceCachedGetRoute(optimizer, userendpoints.GetUserByUsername,
			userhandler.GetUserByUsernameWithCacheInvalidation()),
		userResourceCachedGetRoute(optimizer, userendpoints.GetUserByEmail,
			userhandler.GetUserByEmailWithCacheInvalidation()),
		userResourceCachedGetRoute(optimizer, userendpoints.GetUserByID,
			userhandler.GetUserByIDWithCacheInvalidation()),
		router.CreateRoute(base.Get, userendpoints.GetUserByIdentity,
			userhandler.GetUserByIdentityWithCacheInvalidation()),
		router.CreateRoute(base.Patch, userendpoints.PatchUserByID,
			performance.NewDynamicOptimizedHandlerFunc(
				userhandler.PatchUserByIDWithCacheInvalidation(optimizer),
				constants.ResourceUser,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, userendpoints.DeleteUserByID,
			performance.NewDynamicOptimizedHandlerFunc(
				userhandler.DeleteUserByIDWithCacheInvalidation(optimizer),
				constants.ResourceUser,
				constants.OpDelete,
			),
		),
	}
}

func groupRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, groupendpoints.CreateGroup,
			performance.NewDynamicOptimizedHandlerFunc(
				grouphandler.CreateGroupResourceWithCacheInvalidation(optimizer),
				constants.ResourceGroup,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, groupendpoints.GetAllGroups,
			performance.NewCachedListHandlerFunc(
				optimizer,
				grouphandler.ListGroupResourcesWithCacheInvalidation(),
				exporterauthz.RestrictedKey(cache.NewListCacheKeyFunc(optimizer, constants.ResourceGroup)),
				constants.ResourceGroup,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, groupendpoints.GetGroupByID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				grouphandler.GetGroupByIDWithCacheInvalidation(),
				exporterauthz.RestrictedKey(cache.NewGetCacheKeyFunc(constants.ResourceGroup)),
				constants.ResourceGroup,
				constants.OpGet,
			)),
		router.CreateRoute(base.Patch, groupendpoints.PatchGroupByID,
			performance.NewDynamicOptimizedHandlerFunc(
				grouphandler.PatchGroupByIDWithCacheInvalidation(optimizer),
				constants.ResourceGroup,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, groupendpoints.DeleteGroupByID,
			performance.NewDynamicOptimizedHandlerFunc(
				grouphandler.DeleteGroupByIDWithCacheInvalidation(optimizer),
				constants.ResourceGroup,
				constants.OpDelete,
			),
		),
	}
}

func sessionRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, authendpoints.CreateSessionByUser,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.CreateSessionByUserWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpCreate,
			),
		),
		// Uncached: a hit would be served before the handler's owner check runs.
		router.CreateRoute(base.Get, authendpoints.GetAllSessionsByUser,
			sessionhandler.ListSessionsByUserWithCacheInvalidation()),
		router.CreateRoute(base.Get, authendpoints.GetSelfSession,
			sessionhandler.GetSelfSession()),
		router.CreateRoute(base.Patch, authendpoints.PatchSelfSession,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.PatchSelfSessionWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, authendpoints.DeleteSelfSession,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.DeleteSelfSessionWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpDelete,
			),
		),
		// After the self route: mux takes the first match, and "self" also fits {name}.
		router.CreateRoute(base.Delete, authendpoints.DeleteSessionByName,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.DeleteSessionByNameWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpDelete,
			),
		),
	}
}

func categoryRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, categoryendpoints.CreateCategory,
			performance.NewDynamicOptimizedHandlerFunc(
				categoryhandler.CreateCategoryResourceWithCacheInvalidation(optimizer),
				constants.ResourceCategory,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, categoryendpoints.GetAllCategories,
			performance.NewCachedListHandlerFunc(
				optimizer,
				categoryhandler.ListCategoriesWithCacheInvalidation(),
				cache.NewQueryListCacheKeyFunc(optimizer, constants.ResourceCategory, categoryendpoints.QueryScope),
				constants.ResourceCategory,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, categoryendpoints.GetCategoryByID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				categoryhandler.GetCategoryByIDWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceCategory),
				constants.ResourceCategory,
				constants.OpGet,
			)),
		router.CreateRoute(base.Patch, categoryendpoints.PatchCategoryByID,
			performance.NewDynamicOptimizedHandlerFunc(
				categoryhandler.PatchCategoryByIDWithCacheInvalidation(optimizer),
				constants.ResourceCategory,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, categoryendpoints.DeleteCategoryByID,
			performance.NewDynamicOptimizedHandlerFunc(
				categoryhandler.DeleteCategoryByIDWithCacheInvalidation(optimizer),
				constants.ResourceCategory,
				constants.OpDelete,
			),
		),
	}
}

func roleQueryRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Get, roleendpoints.GetAllAccessRoles,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.ListRoleResourcesWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(optimizer, constants.ResourceRole),
				constants.ResourceRole,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetAccessRoleByID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.GetRoleByIDWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceRole),
				constants.ResourceRole,
				constants.OpGet,
			)),
	}
}

func roleRoutes(optimizer *performance.Optimizer) []router.Route {
	routes := roleQueryRoutes(optimizer)
	return append(routes,
		router.CreateRoute(base.Post, roleendpoints.CreateAccessRole,
			performance.NewDynamicOptimizedHandlerFunc(
				rolehandler.CreateRoleResourceWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Patch, roleendpoints.PatchAccessRoleByID,
			performance.NewDynamicOptimizedHandlerFunc(
				rolehandler.PatchRoleByIDWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, roleendpoints.DeleteAccessRoleByID,
			performance.NewDynamicOptimizedHandlerFunc(
				rolehandler.DeleteRoleByIDWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpDelete,
			),
		),
	)
}

func passkeyRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, authendpoints.CreateInternalPasskeyByUser,
			performance.NewDynamicOptimizedHandlerFunc(
				passkeyhandler.CreatePasskeyByUserWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpCreate,
			),
		),
		subjectListCachedRoute(optimizer, authendpoints.GetAllInternalPasskeysByUser,
			passkeyhandler.ListPasskeysByUserWithCacheInvalidation(),
			constants.ResourceUserPasskey,
			cache.SubjectFromHeader(constants.HeaderUserID)),
		router.CreateRoute(base.Get, authendpoints.GetInternalPasskeyByUserAndCredentialID,
			passkeyhandler.GetPasskeyByUserAndCredentialIDWithCacheInvalidation()),
		router.CreateRoute(base.Patch, authendpoints.PatchInternalPasskeyByUserAndCredentialID,
			performance.NewDynamicOptimizedHandlerFunc(
				passkeyhandler.PatchPasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, authendpoints.DeleteInternalPasskeyByUserAndCredentialID,
			performance.NewDynamicOptimizedHandlerFunc(
				passkeyhandler.DeletePasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpDelete,
			),
		),
	}
}

func snapshotRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, snapshotendpoints.CreateSnapshot,
			performance.NewDynamicOptimizedHandlerFunc(
				snapshothandler.CreateSnapshot(),
				constants.ResourceSnapshot,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshot,
			performance.NewDynamicOptimizedHandlerFunc(
				snapshothandler.GetSnapshot(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshotManifest,
			performance.NewDynamicOptimizedHandlerFunc(
				snapshothandler.GetSnapshotManifest(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshotInfos,
			performance.NewDynamicOptimizedHandlerFunc(
				snapshothandler.GetSnapshotInfos(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Delete, snapshotendpoints.DeleteSnapshot,
			performance.NewDynamicOptimizedHandlerFunc(
				snapshothandler.DeleteSnapshot(),
				constants.ResourceSnapshot,
				constants.OpDelete,
			),
		),
	}
}
