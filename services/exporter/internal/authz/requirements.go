package authz

import (
	roledata "github.com/telark/data/resources/role"
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
	"github.com/telark/x-ware/authz"
)

var (
	public        = authz.Requirement{Access: authz.AccessPublic}
	internal      = authz.Requirement{Access: authz.AccessInternal}
	authenticated = authz.Requirement{Access: authz.AccessAuthenticated}
)

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

// Requirements covers every route InitRoutes registers. A route absent from
// this map is denied, so TestRequirementsCoverEveryRoute fails the build rather
// than letting a new endpoint ship unprotected.
func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addUsers(requirements)
	addGroups(requirements)
	addRoles(requirements)
	addCategories(requirements)
	addApplications(requirements)
	addSnapshots(requirements)
	addPlans(requirements)
	addNotifications(requirements)
	addGlobalConfig(requirements)
	addCleanup(requirements)
	addSessions(requirements)
	addPasskeys(requirements)

	return requirements
}

// Called by kubelet, which holds no session. Denying these fails the probes and
// the pod never becomes ready.
func addStatus(r map[string]authz.Requirement) {
	r[router.Key(base.Get, statusendpoints.LivenessCheck)] = public
	r[router.Key(base.Get, statusendpoints.ReadinessCheck)] = public
}

func addUsers(r map[string]authz.Requirement) {
	r[router.Key(base.Post, userendpoints.CreateUser)] = denyable(
		write(roledata.ScopeUsers), roledata.ActionCreateUser)
	r[router.Key(base.Get, userendpoints.GetAllUsers)] = read(roledata.ScopeUsers)
	r[router.Key(base.Get, userendpoints.GetUserByID)] = read(roledata.ScopeUsers)
	r[router.Key(base.Get, userendpoints.GetUserByEmail)] = read(roledata.ScopeUsers)
	// The role, group and status fields each have their own deny rule, so this
	// route carries none: the field guard checks them per field instead.
	r[router.Key(base.Patch, userendpoints.PatchUserByID)] = write(roledata.ScopeUsers)
	r[router.Key(base.Delete, userendpoints.DeleteUserByID)] = denyable(
		remove(roledata.ScopeUsers), roledata.ActionDeleteUser)

	// Login-only lookups: the auth service resolves an identity before any
	// session exists, so no session can be required here.
	r[router.Key(base.Get, userendpoints.GetUserByUsername)] = internal
	r[router.Key(base.Get, userendpoints.GetUserByIdentity)] = internal
}

func addGroups(r map[string]authz.Requirement) {
	r[router.Key(base.Post, groupendpoints.CreateGroup)] = denyable(
		write(roledata.ScopeGroups), roledata.ActionCreateGroup)
	r[router.Key(base.Get, groupendpoints.GetAllGroups)] = read(roledata.ScopeGroups)
	r[router.Key(base.Get, groupendpoints.GetGroupByID)] = read(roledata.ScopeGroups)
	r[router.Key(base.Patch, groupendpoints.PatchGroupByID)] = denyable(
		write(roledata.ScopeGroups), roledata.ActionEditGroup)
	r[router.Key(base.Delete, groupendpoints.DeleteGroupByID)] = denyable(
		remove(roledata.ScopeGroups), roledata.ActionDeleteGroup)
}

func addRoles(r map[string]authz.Requirement) {
	r[router.Key(base.Post, roleendpoints.CreateRole)] = denyable(
		write(roledata.ScopeRoles), roledata.ActionCreateRole)
	r[router.Key(base.Get, roleendpoints.GetAllRoles)] = read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByID)] = read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByUserID)] = read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRolesByUserID)] = read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByGroupID)] = read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRolesByGroupID)] = read(roledata.ScopeRoles)
	r[router.Key(base.Patch, roleendpoints.PatchRoleByID)] = denyable(
		write(roledata.ScopeRoles), roledata.ActionEditRole)
	r[router.Key(base.Delete, roleendpoints.DeleteRoleByID)] = denyable(
		remove(roledata.ScopeRoles), roledata.ActionDeleteRole)
}

// A category names the scope it classifies, so no single scope governs these
// routes. Reads are reference data any signed-in user needs to render a list;
// writes defer to GuardCategoryScope, which resolves the scope from the
// category itself.
func addCategories(r map[string]authz.Requirement) {
	r[router.Key(base.Get, categoryendpoints.GetAllCategories)] = authenticated
	r[router.Key(base.Get, categoryendpoints.GetCategoryByID)] = authenticated
	r[router.Key(base.Get, categoryendpoints.GetCategoriesByScope)] = authenticated
	r[router.Key(base.Post, categoryendpoints.CreateCategory)] = authenticated
	r[router.Key(base.Patch, categoryendpoints.PatchCategoryByID)] = authenticated
	r[router.Key(base.Delete, categoryendpoints.DeleteCategoryByID)] = authenticated
}

func addApplications(r map[string]authz.Requirement) {
	r[router.Key(base.Post, applicationendpoints.CreateApplication)] = write(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetAllApplications)] = read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetApplicationByName)] = read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetRollbacks)] = denyable(
		read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks)
	r[router.Key(base.Get, applicationendpoints.GetRollback)] = denyable(
		read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks)
	r[router.Key(base.Patch, applicationendpoints.PatchApplicationByName)] = denyable(
		write(roledata.ScopeApplications), roledata.ActionEditApplication)
	r[router.Key(base.Delete, applicationendpoints.DeleteApplicationByName)] = denyable(
		remove(roledata.ScopeApplications), roledata.ActionDeleteApplication)
}

// Snapshots are taken of applications, so they follow the applications scope.
func addSnapshots(r map[string]authz.Requirement) {
	r[router.Key(base.Post, snapshotendpoints.CreateSnapshot)] = write(roledata.ScopeApplications)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshot)] = denyable(
		read(roledata.ScopeApplications), roledata.ActionViewApplicationsSnapshots)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotInfos)] = denyable(
		read(roledata.ScopeApplications), roledata.ActionViewApplicationsSnapshots)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotManifest)] = denyable(
		read(roledata.ScopeApplications), roledata.ActionViewApplicationSnapshotManifest)
	r[router.Key(base.Delete, snapshotendpoints.DeleteSnapshot)] = remove(roledata.ScopeApplications)
}

// Protection plans are their own feature with their own scope: managing an
// application does not imply managing the plans that govern it.
func addPlans(r map[string]authz.Requirement) {
	r[router.Key(base.Get, plansendpoints.ListProtectionPlans)] = read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, plansendpoints.GetProtectionPlanByID)] = read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, plansendpoints.CreateProtectionPlan)] = denyable(
		write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan)
	r[router.Key(base.Patch, plansendpoints.PatchProtectionPlanByID)] = denyable(
		write(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlan)
	r[router.Key(base.Delete, plansendpoints.DeleteProtectionPlanByID)] = denyable(
		remove(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlan)
}

// A user's own notifications: no scope, because every user must reach these
// regardless of which scopes their roles happen to carry.
func addNotifications(r map[string]authz.Requirement) {
	r[router.Key(base.Get, notificationsendpoints.List)] = authenticated
	r[router.Key(base.Patch, notificationsendpoints.MarkRead)] = authenticated
	r[router.Key(base.Post, notificationsendpoints.MarkAllRead)] = authenticated
	r[router.Key(base.Delete, notificationsendpoints.Clear)] = authenticated

	// Emitted by peer services reacting to cluster events, not by users.
	r[router.Key(base.Post, notificationsendpoints.Emit)] = internal
}

// One endpoint patches the whole config, and its parts are not governed alike:
// display preferences and the reported cluster version are nobody's privilege,
// while discovery config, snapshot storage, AI and identity settings each have
// their own. No single route rule fits, so GuardGlobalConfigPatch checks the
// fields the body actually touches.
func addGlobalConfig(r map[string]authz.Requirement) {
	r[router.Key(base.Get, globalconfigendpoints.GetGlobalConfig)] = read(roledata.ScopeSettings)
	r[router.Key(base.Patch, globalconfigendpoints.PatchGlobalConfig)] = authenticated
}

// Finalizer edits decide whether a resource can be deleted at all.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Patch, cleanupendpoints.AddFinalizer)] = remove(roledata.ScopeSettings)
	r[router.Key(base.Patch, cleanupendpoints.RemoveFinalizer)] = remove(roledata.ScopeSettings)
	r[router.Key(base.Get, cleanupendpoints.GetCleanupViewByID)] = read(roledata.ScopeSettings)
	r[router.Key(base.Get, cleanupendpoints.ListCleanupViews)] = read(roledata.ScopeSettings)
}

// Managing one's own sessions is not an administrative act, so it takes no
// scope. The handler guards narrow each route to the caller's own sessions;
// without them any authenticated user could read another user's live tokens.
func addSessions(r map[string]authz.Requirement) {
	r[router.Key(base.Get, authendpoints.GetAllSessionsByUser)] = authenticated
	r[router.Key(base.Get, authendpoints.GetSessionByToken)] = authenticated
	r[router.Key(base.Delete, authendpoints.DeleteSessionByToken)] = authenticated

	r[router.Key(base.Post, authendpoints.CreateSessionByUser)] = internal
	r[router.Key(base.Patch, authendpoints.PatchSessionByToken)] = internal
}

// Reached only through the auth service's passkey proxy, during a WebAuthn
// ceremony that by definition has no session yet.
func addPasskeys(r map[string]authz.Requirement) {
	r[router.Key(base.Post, authendpoints.CreateInternalPasskeyByUser)] = internal
	r[router.Key(base.Get, authendpoints.GetAllInternalPasskeysByUser)] = internal
	r[router.Key(base.Get, authendpoints.GetInternalPasskeyByUserAndCredentialID)] = internal
	r[router.Key(base.Patch, authendpoints.PatchInternalPasskeyByUserAndCredentialID)] = internal
	r[router.Key(base.Delete, authendpoints.DeleteInternalPasskeyByUserAndCredentialID)] = internal
}
