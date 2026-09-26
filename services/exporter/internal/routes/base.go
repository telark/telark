package routes

import (
	"net/http"

	exporterauthz "github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/cache"
	"github.com/telark/exporter/internal/constants"
	passkeyhandler "github.com/telark/exporter/internal/handlers/auth/passkey"
	sessionhandler "github.com/telark/exporter/internal/handlers/auth/session"
	categoryhandler "github.com/telark/exporter/internal/handlers/classification/category"
	notificationhandler "github.com/telark/exporter/internal/handlers/notifications"
	planshandler "github.com/telark/exporter/internal/handlers/plans/protection"
	reportshandler "github.com/telark/exporter/internal/handlers/reports"
	applicationhandler "github.com/telark/exporter/internal/handlers/resources/application"
	cleanuphandler "github.com/telark/exporter/internal/handlers/resources/cleanup"
	globalconfighandler "github.com/telark/exporter/internal/handlers/resources/globalconfig"
	grouphandler "github.com/telark/exporter/internal/handlers/resources/group"
	rolehandler "github.com/telark/exporter/internal/handlers/resources/role"
	userhandler "github.com/telark/exporter/internal/handlers/resources/user"
	snapshothandler "github.com/telark/exporter/internal/handlers/snapshot"
	statushandler "github.com/telark/exporter/internal/handlers/status"
	"github.com/telark/exporter/internal/utils/performance"
	"github.com/telark/rest/base"
	authendpoints "github.com/telark/rest/endpoints/auth"
	categoryendpoints "github.com/telark/rest/endpoints/classification/category"
	notificationsendpoints "github.com/telark/rest/endpoints/notifications"
	plansendpoints "github.com/telark/rest/endpoints/plans"
	reportsendpoints "github.com/telark/rest/endpoints/reports"
	applicationendpoints "github.com/telark/rest/endpoints/resources/applications"
	cleanupendpoints "github.com/telark/rest/endpoints/resources/cleanup"
	globalconfigendpoints "github.com/telark/rest/endpoints/resources/globalconfig"
	groupendpoints "github.com/telark/rest/endpoints/resources/groups"
	roleendpoints "github.com/telark/rest/endpoints/resources/roles"
	userendpoints "github.com/telark/rest/endpoints/resources/users"
	snapshotendpoints "github.com/telark/rest/endpoints/snapshots"
	statusendpoints "github.com/telark/rest/endpoints/status"
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
	routes = append(routes, globalConfigRoutes()...)
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
		router.CreateRoute(base.Patch, cleanupendpoints.AddFinalizer, cleanuphandler.AddFinalizer),
		router.CreateRoute(base.Patch, cleanupendpoints.RemoveFinalizer, cleanuphandler.RemoveFinalizer),
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
		router.CreateRoute(base.Post, reportsendpoints.PutPlanReportLedger, reportshandler.PutPlanReportLedger()),
		router.CreateRoute(base.Get, reportsendpoints.GetPlanReportLedger, reportshandler.GetPlanReportLedger()),
	}
}

func notificationRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, notificationsendpoints.Emit, notificationhandler.Emit()),
		router.CreateRoute(base.Get, notificationsendpoints.List, notificationhandler.List()),
		router.CreateRoute(base.Patch, notificationsendpoints.MarkRead, notificationhandler.MarkRead()),
		router.CreateRoute(base.Post, notificationsendpoints.MarkAllRead, notificationhandler.MarkAllRead()),
		router.CreateRoute(base.Delete, notificationsendpoints.Clear, notificationhandler.Clear()),
	}
}

func globalConfigRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Get, globalconfigendpoints.GetGlobalConfig, globalconfighandler.GetGlobalConfig()),
		router.CreateRoute(base.Patch, globalconfigendpoints.PatchGlobalConfig, globalconfighandler.PatchGlobalConfig()),
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
		subjectListCachedRoute(optimizer, authendpoints.GetAllSessionsByUser,
			sessionhandler.ListSessionsByUserWithCacheInvalidation(),
			constants.ResourceUserSession,
			cache.SubjectFromPathParam(constants.UserIDParam)),
		router.CreateRoute(base.Get, authendpoints.GetSessionByToken,
			sessionhandler.GetSessionByToken()),
		router.CreateRoute(base.Patch, authendpoints.PatchSessionByToken,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.PatchSessionByTokenWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, authendpoints.DeleteSessionByToken,
			performance.NewDynamicOptimizedHandlerFunc(
				sessionhandler.DeleteSessionByTokenWithCacheInvalidation(optimizer),
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
				categoryhandler.ListAllCategoriesWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(optimizer, constants.ResourceCategory),
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
		router.CreateRoute(base.Get, categoryendpoints.GetCategoriesByScope,
			performance.NewCachedListHandlerFunc(
				optimizer,
				categoryhandler.GetCategoriesByScopeWithCacheInvalidation(),
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
		router.CreateRoute(base.Get, roleendpoints.GetAllRoles,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.ListRoleResourcesWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(optimizer, constants.ResourceRole),
				constants.ResourceRole,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetRoleByID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.GetRoleByIDWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceRole),
				constants.ResourceRole,
				constants.OpGet,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetRoleByUserID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.GetRoleByUserIDWithCacheInvalidation(),
				exporterauthz.RestrictedKey(cache.NewGetCacheKeyFunc(constants.ResourceRole)),
				constants.ResourceRole,
				constants.OpGet,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetRolesByUserID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.ListRolesByUserIDWithCacheInvalidation(),
				exporterauthz.RestrictedKey(cache.NewSubjectListCacheKeyFunc(
					optimizer, constants.ResourceRole, cache.SubjectFromPathParam(constants.UserIDParam),
				)),
				constants.ResourceRole,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetRoleByGroupID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.GetRoleByGroupIDWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceRole),
				constants.ResourceRole,
				constants.OpGet,
			)),
		subjectListCachedRoute(optimizer, roleendpoints.GetRolesByGroupID,
			rolehandler.ListRolesByGroupIDWithCacheInvalidation(),
			constants.ResourceRole,
			cache.SubjectFromPathParam(constants.GroupIDParam)),
	}
}

func roleRoutes(optimizer *performance.Optimizer) []router.Route {
	routes := roleQueryRoutes(optimizer)
	return append(routes,
		router.CreateRoute(base.Post, roleendpoints.CreateRole,
			performance.NewDynamicOptimizedHandlerFunc(
				rolehandler.CreateRoleResourceWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Patch, roleendpoints.PatchRoleByID,
			performance.NewDynamicOptimizedHandlerFunc(
				rolehandler.PatchRoleByIDWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, roleendpoints.DeleteRoleByID,
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
