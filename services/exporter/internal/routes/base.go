package routes

import (
	"net/http"

	"github.com/telark/exporter/cache"
	"github.com/telark/exporter/constants"
	passkeyhandler "github.com/telark/exporter/handlers/auth/passkey"
	sessionhandler "github.com/telark/exporter/handlers/auth/session"
	categoryhandler "github.com/telark/exporter/handlers/classification/category"
	notificationhandler "github.com/telark/exporter/handlers/notifications"
	planshandler "github.com/telark/exporter/handlers/plans/protection"
	applicationhandler "github.com/telark/exporter/handlers/resources/application"
	cleanuphandler "github.com/telark/exporter/handlers/resources/cleanup"
	globalconfighandler "github.com/telark/exporter/handlers/resources/globalconfig"
	grouphandler "github.com/telark/exporter/handlers/resources/group"
	rolehandler "github.com/telark/exporter/handlers/resources/role"
	userhandler "github.com/telark/exporter/handlers/resources/user"
	snapshothandler "github.com/telark/exporter/handlers/snapshot"
	statushandler "github.com/telark/exporter/handlers/status"
	"github.com/telark/exporter/utils/performance"
	"github.com/telark/rest/base"
	authendpoints "github.com/telark/rest/endpoints/auth"
	categoryendpoints "github.com/telark/rest/endpoints/classification/category"
	notificationsendpoints "github.com/telark/rest/endpoints/notifications"
	plansendpoints "github.com/telark/rest/endpoints/plans"
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
			cache.NewGetCacheKeyFunc(constants.ResourceUser),
			constants.ResourceUser,
			constants.OpGet,
		))
}

func InitRoutes(optimizer *performance.Optimizer) []router.Route {
	routes := make([]router.Route, constants.DefaultInitValue, constants.DefaultRoutesCount)
	routes = append(routes, applicationRoutes(optimizer)...)
	routes = append(routes, globalConfigRoutes(optimizer)...)
	routes = append(routes, userRoutes(optimizer)...)
	routes = append(routes, groupRoutes(optimizer)...)
	routes = append(routes, roleRoutes(optimizer)...)
	routes = append(routes, categoryRoutes(optimizer)...)
	routes = append(routes, sessionRoutes(optimizer)...)
	routes = append(routes, passkeyRoutes(optimizer)...)
	routes = append(routes, snapshotRoutes(optimizer)...)
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
		router.CreateRoute(base.Get, statusendpoints.LivenessCheck, statushandler.ProbeHandler),
		router.CreateRoute(base.Get, statusendpoints.ReadinessCheck, statushandler.ProbeHandler),
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

func notificationRoutes() []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, notificationsendpoints.Emit, notificationhandler.Emit()),
		router.CreateRoute(base.Get, notificationsendpoints.List, notificationhandler.List()),
		router.CreateRoute(base.Patch, notificationsendpoints.MarkRead, notificationhandler.MarkRead()),
		router.CreateRoute(base.Post, notificationsendpoints.MarkAllRead, notificationhandler.MarkAllRead()),
		router.CreateRoute(base.Delete, notificationsendpoints.Clear, notificationhandler.Clear()),
	}
}

func globalConfigRoutes(optimizer *performance.Optimizer) []router.Route {
	_ = optimizer
	return []router.Route{
		router.CreateRoute(base.Get, globalconfigendpoints.GetGlobalConfig, globalconfighandler.GetGlobalConfig()),
		router.CreateRoute(base.Patch, globalconfigendpoints.PatchGlobalConfig, globalconfighandler.PatchGlobalConfig()),
	}
}

func applicationRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, applicationendpoints.CreateApplication,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				applicationhandler.CreateApplicationResourceWithCacheInvalidation(optimizer),
				constants.ResourceApplication,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetAllApplications,
			performance.NewCachedListHandlerFunc(
				optimizer,
				applicationhandler.ListApplicationResourcesWithCacheInvalidation(optimizer),
				cache.NewListCacheKeyFunc(constants.ResourceApplication),
				constants.ResourceApplication,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, applicationendpoints.GetRollbacks,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				applicationhandler.GetRollbacks(),
				constants.ResourceApplication,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetRollback,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				applicationhandler.GetRollback(),
				constants.ResourceApplication,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, applicationendpoints.GetApplicationByName,
			performance.NewCachedListHandlerFunc(
				optimizer,
				applicationhandler.GetApplicationResourceWithCacheInvalidation(optimizer),
				cache.NewGetCacheKeyFunc(constants.ResourceApplication),
				constants.ResourceApplication,
				constants.OpGet,
			)),
		router.CreateRoute(base.Patch, applicationendpoints.PatchApplicationByName,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				applicationhandler.PatchApplicationResourceWithCacheInvalidation(optimizer),
				constants.ResourceApplication,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, applicationendpoints.DeleteApplicationByName,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				optimizer,
				userhandler.CreateUserResourceWithCacheInvalidation(optimizer),
				constants.ResourceUser,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, userendpoints.GetAllUsers,
			performance.NewCachedListHandlerFunc(
				optimizer,
				userhandler.ListUserResourcesWithCacheInvalidation(optimizer),
				cache.NewListCacheKeyFunc(constants.ResourceUser),
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
				optimizer,
				userhandler.PatchUserByIDWithCacheInvalidation(optimizer),
				constants.ResourceUser,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, userendpoints.DeleteUserByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				optimizer,
				grouphandler.CreateGroupResourceWithCacheInvalidation(optimizer),
				constants.ResourceGroup,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, groupendpoints.GetAllGroups,
			performance.NewCachedListHandlerFunc(
				optimizer,
				grouphandler.ListGroupResourcesWithCacheInvalidation(optimizer),
				cache.NewListCacheKeyFunc(constants.ResourceGroup),
				constants.ResourceGroup,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, groupendpoints.GetGroupByID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				grouphandler.GetGroupByIDWithCacheInvalidation(),
				cache.NewGetCacheKeyFunc(constants.ResourceGroup),
				constants.ResourceGroup,
				constants.OpGet,
			)),
		router.CreateRoute(base.Patch, groupendpoints.PatchGroupByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				grouphandler.PatchGroupByIDWithCacheInvalidation(optimizer),
				constants.ResourceGroup,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, groupendpoints.DeleteGroupByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				optimizer,
				sessionhandler.CreateSessionByUserWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, authendpoints.GetAllSessionsByUser,
			performance.NewCachedListHandlerFunc(
				optimizer,
				sessionhandler.ListSessionsByUserWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(constants.ResourceUserSession),
				constants.ResourceUserSession,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, authendpoints.GetSessionByToken,
			sessionhandler.GetSessionByToken()),
		router.CreateRoute(base.Patch, authendpoints.PatchSessionByToken,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				sessionhandler.PatchSessionByTokenWithCacheInvalidation(optimizer),
				constants.ResourceUserSession,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, authendpoints.DeleteSessionByToken,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				optimizer,
				categoryhandler.CreateCategoryResourceWithCacheInvalidation(optimizer),
				constants.ResourceCategory,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, categoryendpoints.GetAllCategories,
			performance.NewCachedListHandlerFunc(
				optimizer,
				categoryhandler.ListAllCategoriesWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(constants.ResourceCategory),
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
				optimizer,
				categoryhandler.PatchCategoryByIDWithCacheInvalidation(optimizer),
				constants.ResourceCategory,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, categoryendpoints.DeleteCategoryByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				rolehandler.ListRoleResourcesWithCacheInvalidation(optimizer),
				cache.NewListCacheKeyFunc(constants.ResourceRole),
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
				cache.NewGetCacheKeyFunc(constants.ResourceRole),
				constants.ResourceRole,
				constants.OpGet,
			)),
		router.CreateRoute(base.Get, roleendpoints.GetRolesByUserID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.ListRolesByUserIDWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(constants.ResourceRole),
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
		router.CreateRoute(base.Get, roleendpoints.GetRolesByGroupID,
			performance.NewCachedListHandlerFunc(
				optimizer,
				rolehandler.ListRolesByGroupIDWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(constants.ResourceRole),
				constants.ResourceRole,
				constants.OpList,
			)),
	}
}

func roleRoutes(optimizer *performance.Optimizer) []router.Route {
	routes := roleQueryRoutes(optimizer)
	return append(routes,
		router.CreateRoute(base.Post, roleendpoints.CreateRole,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				rolehandler.CreateRoleResourceWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Patch, roleendpoints.PatchRoleByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				rolehandler.PatchRoleByIDWithCacheInvalidation(optimizer),
				constants.ResourceRole,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, roleendpoints.DeleteRoleByID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
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
				optimizer,
				passkeyhandler.CreatePasskeyByUserWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, authendpoints.GetAllInternalPasskeysByUser,
			performance.NewCachedListHandlerFunc(
				optimizer,
				passkeyhandler.ListPasskeysByUserWithCacheInvalidation(),
				cache.NewListCacheKeyFunc(constants.ResourceUserPasskey),
				constants.ResourceUserPasskey,
				constants.OpList,
			)),
		router.CreateRoute(base.Get, authendpoints.GetInternalPasskeyByUserAndCredentialID,
			passkeyhandler.GetPasskeyByUserAndCredentialIDWithCacheInvalidation()),
		router.CreateRoute(base.Patch, authendpoints.PatchInternalPasskeyByUserAndCredentialID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				passkeyhandler.PatchPasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpPatch,
			),
		),
		router.CreateRoute(base.Delete, authendpoints.DeleteInternalPasskeyByUserAndCredentialID,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				passkeyhandler.DeletePasskeyByUserAndCredentialIDWithCacheInvalidation(optimizer),
				constants.ResourceUserPasskey,
				constants.OpDelete,
			),
		),
	}
}

func snapshotRoutes(optimizer *performance.Optimizer) []router.Route {
	return []router.Route{
		router.CreateRoute(base.Post, snapshotendpoints.CreateSnapshot,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				snapshothandler.CreateSnapshot(),
				constants.ResourceSnapshot,
				constants.OpCreate,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshot,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				snapshothandler.GetSnapshot(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshotManifest,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				snapshothandler.GetSnapshotManifest(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
		router.CreateRoute(base.Get, snapshotendpoints.GetSnapshotInfos,
			performance.NewDynamicOptimizedHandlerFunc(
				optimizer,
				snapshothandler.GetSnapshotInfos(),
				constants.ResourceSnapshot,
				constants.OpGet,
			),
		),
	}
}
