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

// A route absent from this map is denied; the coverage test enforces that.
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

// Kubelet holds no session; denying these means the pod never becomes ready.
func addStatus(r map[string]authz.Requirement) {
	r[router.Key(base.Get, statusendpoints.LivenessCheck)] = authz.Public
	r[router.Key(base.Get, statusendpoints.ReadinessCheck)] = authz.Public
}

func addUsers(r map[string]authz.Requirement) {
	r[router.Key(base.Post, userendpoints.CreateUser)] = authz.Denyable(
		authz.Write(roledata.ScopeUsers), roledata.ActionCreateUser)
	r[router.Key(base.Get, userendpoints.GetAllUsers)] = authz.Read(roledata.ScopeUsers)
	r[router.Key(base.Get, userendpoints.GetUserByID)] = authz.Read(roledata.ScopeUsers)
	r[router.Key(base.Get, userendpoints.GetUserByEmail)] = authz.Read(roledata.ScopeUsers)
	// No rule here: each privileged field has its own, checked by the guard.
	r[router.Key(base.Patch, userendpoints.PatchUserByID)] = authz.Write(roledata.ScopeUsers)
	r[router.Key(base.Delete, userendpoints.DeleteUserByID)] = authz.Denyable(
		authz.Own(roledata.ScopeUsers), roledata.ActionDeleteUser)

	// Login resolves an identity before a session exists.
	r[router.Key(base.Get, userendpoints.GetUserByUsername)] = authz.Internal
	r[router.Key(base.Get, userendpoints.GetUserByIdentity)] = authz.Internal
}

func addGroups(r map[string]authz.Requirement) {
	r[router.Key(base.Post, groupendpoints.CreateGroup)] = authz.Denyable(
		authz.Write(roledata.ScopeGroups), roledata.ActionCreateGroup)
	r[router.Key(base.Get, groupendpoints.GetAllGroups)] = authz.Read(roledata.ScopeGroups)
	r[router.Key(base.Get, groupendpoints.GetGroupByID)] = authz.Read(roledata.ScopeGroups)
	r[router.Key(base.Patch, groupendpoints.PatchGroupByID)] = authz.Denyable(
		authz.Write(roledata.ScopeGroups), roledata.ActionEditGroup)
	r[router.Key(base.Delete, groupendpoints.DeleteGroupByID)] = authz.Denyable(
		authz.Own(roledata.ScopeGroups), roledata.ActionDeleteGroup)
}

func addRoles(r map[string]authz.Requirement) {
	r[router.Key(base.Post, roleendpoints.CreateRole)] = authz.Denyable(
		authz.Write(roledata.ScopeRoles), roledata.ActionCreateRole)
	r[router.Key(base.Get, roleendpoints.GetAllRoles)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByUserID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRolesByUserID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRoleByGroupID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetRolesByGroupID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Patch, roleendpoints.PatchRoleByID)] = authz.Denyable(
		authz.Write(roledata.ScopeRoles), roledata.ActionEditRole)
	r[router.Key(base.Delete, roleendpoints.DeleteRoleByID)] = authz.Denyable(
		authz.Own(roledata.ScopeRoles), roledata.ActionDeleteRole)
}

// A category names the scope it classifies, so no route rule fits; writes
// defer to GuardCategoryScope.
func addCategories(r map[string]authz.Requirement) {
	r[router.Key(base.Get, categoryendpoints.GetAllCategories)] = authz.Authenticated
	r[router.Key(base.Get, categoryendpoints.GetCategoryByID)] = authz.Authenticated
	r[router.Key(base.Get, categoryendpoints.GetCategoriesByScope)] = authz.Authenticated
	r[router.Key(base.Post, categoryendpoints.CreateCategory)] = authz.Authenticated
	r[router.Key(base.Patch, categoryendpoints.PatchCategoryByID)] = authz.Authenticated
	r[router.Key(base.Delete, categoryendpoints.DeleteCategoryByID)] = authz.Authenticated
}

func addApplications(r map[string]authz.Requirement) {
	r[router.Key(base.Post, applicationendpoints.CreateApplication)] = authz.Write(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetAllApplications)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetApplicationByName)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetRollbacks)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks)
	r[router.Key(base.Get, applicationendpoints.GetRollback)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks)
	r[router.Key(base.Patch, applicationendpoints.PatchApplicationByName)] = authz.Denyable(
		authz.Write(roledata.ScopeApplications), roledata.ActionEditApplication)
	r[router.Key(base.Delete, applicationendpoints.DeleteApplicationByName)] = authz.Denyable(
		authz.Own(roledata.ScopeApplications), roledata.ActionDeleteApplication)
}

// Snapshots are taken of applications, so they follow the applications scope.
func addSnapshots(r map[string]authz.Requirement) {
	r[router.Key(base.Post, snapshotendpoints.CreateSnapshot)] = authz.Write(roledata.ScopeApplications)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshot)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsSnapshots)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotInfos)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsSnapshots)
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotManifest)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationSnapshotManifest)
	r[router.Key(base.Delete, snapshotendpoints.DeleteSnapshot)] = authz.Own(roledata.ScopeApplications)
}

// Their own feature: managing an application does not imply managing its plans.
func addPlans(r map[string]authz.Requirement) {
	r[router.Key(base.Get, plansendpoints.ListProtectionPlans)] = authz.Read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Get, plansendpoints.GetProtectionPlanByID)] = authz.Read(roledata.ScopeProtectionPlans)
	r[router.Key(base.Post, plansendpoints.CreateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan)
	r[router.Key(base.Patch, plansendpoints.PatchProtectionPlanByID)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlan)
	r[router.Key(base.Delete, plansendpoints.DeleteProtectionPlanByID)] = authz.Denyable(
		authz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlan)
}

// Every user reaches their own, whatever scopes their roles carry.
func addNotifications(r map[string]authz.Requirement) {
	r[router.Key(base.Get, notificationsendpoints.List)] = authz.Authenticated
	r[router.Key(base.Patch, notificationsendpoints.MarkRead)] = authz.Authenticated
	r[router.Key(base.Post, notificationsendpoints.MarkAllRead)] = authz.Authenticated
	r[router.Key(base.Delete, notificationsendpoints.Clear)] = authz.Authenticated

	// Emitted by peer services reacting to cluster events, not by users.
	r[router.Key(base.Post, notificationsendpoints.Emit)] = authz.Internal
}

// Its parts are not governed alike, so GuardGlobalConfigPatch checks per field.
func addGlobalConfig(r map[string]authz.Requirement) {
	r[router.Key(base.Get, globalconfigendpoints.GetGlobalConfig)] = authz.Read(roledata.ScopeSettings)
	r[router.Key(base.Patch, globalconfigendpoints.PatchGlobalConfig)] = authz.Authenticated
}

// Finalizer edits decide whether a resource can be deleted at all.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Patch, cleanupendpoints.AddFinalizer)] = authz.Own(roledata.ScopeSettings)
	r[router.Key(base.Patch, cleanupendpoints.RemoveFinalizer)] = authz.Own(roledata.ScopeSettings)
	r[router.Key(base.Get, cleanupendpoints.GetCleanupViewByID)] = authz.Read(roledata.ScopeSettings)
	r[router.Key(base.Get, cleanupendpoints.ListCleanupViews)] = authz.Read(roledata.ScopeSettings)
}

// Not administrative, so no scope; the guards narrow these to the caller.
func addSessions(r map[string]authz.Requirement) {
	r[router.Key(base.Get, authendpoints.GetAllSessionsByUser)] = authz.Authenticated
	r[router.Key(base.Get, authendpoints.GetSessionByToken)] = authz.Authenticated
	r[router.Key(base.Delete, authendpoints.DeleteSessionByToken)] = authz.Authenticated

	r[router.Key(base.Post, authendpoints.CreateSessionByUser)] = authz.Internal
	r[router.Key(base.Patch, authendpoints.PatchSessionByToken)] = authz.Internal
}

// Reached via the passkey proxy, during a ceremony that has no session yet.
func addPasskeys(r map[string]authz.Requirement) {
	r[router.Key(base.Post, authendpoints.CreateInternalPasskeyByUser)] = authz.Internal
	r[router.Key(base.Get, authendpoints.GetAllInternalPasskeysByUser)] = authz.Internal
	r[router.Key(base.Get, authendpoints.GetInternalPasskeyByUserAndCredentialID)] = authz.Internal
	r[router.Key(base.Patch, authendpoints.PatchInternalPasskeyByUserAndCredentialID)] = authz.Internal
	r[router.Key(base.Delete, authendpoints.DeleteInternalPasskeyByUserAndCredentialID)] = authz.Internal
}
