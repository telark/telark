package authz

import (
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	roleendpoints "github.com/telark/telark/internal/rest/endpoints/accessroles"
	applicationendpoints "github.com/telark/telark/internal/rest/endpoints/applications"
	authendpoints "github.com/telark/telark/internal/rest/endpoints/auth"
	categoryendpoints "github.com/telark/telark/internal/rest/endpoints/categories"
	cleanupendpoints "github.com/telark/telark/internal/rest/endpoints/cleanup"
	configendpoints "github.com/telark/telark/internal/rest/endpoints/config"
	groupendpoints "github.com/telark/telark/internal/rest/endpoints/groups"
	notificationsendpoints "github.com/telark/telark/internal/rest/endpoints/notifications"
	plansendpoints "github.com/telark/telark/internal/rest/endpoints/plans"
	reportsendpoints "github.com/telark/telark/internal/rest/endpoints/reports"
	snapshotendpoints "github.com/telark/telark/internal/rest/endpoints/snapshots"
	statusendpoints "github.com/telark/telark/internal/rest/endpoints/status"
	userendpoints "github.com/telark/telark/internal/rest/endpoints/users"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/internal/x-ware/authz"
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
	addReports(requirements)
	addPlans(requirements)
	addNotifications(requirements)
	addConfig(requirements)
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
		authz.Write(roledata.ScopeUsers), roledata.ActionCreateUser,
	)
	r[router.Key(base.Get, userendpoints.GetAllUsers)] = authz.Read(roledata.ScopeUsers)
	// Everyone reads their own profile; GuardUserRead needs users ReadOnly for anyone else's.
	r[router.Key(base.Get, userendpoints.GetUserByID)] = authz.Authenticated
	// Audit actors are named for every viewer; only id → username leaves.
	r[router.Key(base.Get, userendpoints.GetUserNames)] = authz.Authenticated
	// Any signed-in user may edit their own profile, so the route only needs a
	// session; GuardUserPatch enforces ownership and each privileged field's rule.
	r[router.Key(base.Patch, userendpoints.PatchUserByID)] = authz.Authenticated
	r[router.Key(base.Delete, userendpoints.DeleteUserByID)] = authz.Denyable(
		authz.Own(roledata.ScopeUsers), roledata.ActionDeleteUser,
	)

	// Login resolves an identity before a session exists.
	r[router.Key(base.Get, userendpoints.GetUserByUsername)] = authz.Internal
	r[router.Key(base.Get, userendpoints.GetUserByEmail)] = authz.Internal
	r[router.Key(base.Get, userendpoints.GetUserByIdentity)] = authz.Internal
}

func addGroups(r map[string]authz.Requirement) {
	r[router.Key(base.Post, groupendpoints.CreateGroup)] = authz.Denyable(
		authz.Write(roledata.ScopeGroups), roledata.ActionCreateGroup,
	)
	r[router.Key(base.Get, groupendpoints.GetAllGroups)] = authz.Read(roledata.ScopeGroups)
	r[router.Key(base.Get, groupendpoints.GetGroupByID)] = authz.Read(roledata.ScopeGroups)
	r[router.Key(base.Patch, groupendpoints.PatchGroupByID)] = authz.Denyable(
		authz.Write(roledata.ScopeGroups), roledata.ActionEditGroup,
	)
	r[router.Key(base.Delete, groupendpoints.DeleteGroupByID)] = authz.Denyable(
		authz.Own(roledata.ScopeGroups), roledata.ActionDeleteGroup,
	)
}

func addRoles(r map[string]authz.Requirement) {
	r[router.Key(base.Post, roleendpoints.CreateAccessRole)] = authz.Denyable(
		authz.Write(roledata.ScopeRoles), roledata.ActionCreateRole,
	)
	r[router.Key(base.Get, roleendpoints.GetAllAccessRoles)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Get, roleendpoints.GetAccessRoleByID)] = authz.Read(roledata.ScopeRoles)
	r[router.Key(base.Patch, roleendpoints.PatchAccessRoleByID)] = authz.Denyable(
		authz.Write(roledata.ScopeRoles), roledata.ActionEditRole,
	)
	r[router.Key(base.Delete, roleendpoints.DeleteAccessRoleByID)] = authz.Denyable(
		authz.Own(roledata.ScopeRoles), roledata.ActionDeleteRole,
	)
}

// A category names the scope it classifies (the list also filters by ?scope=), so no route rule fits; writes
// defer to GuardCategoryScope.
func addCategories(r map[string]authz.Requirement) {
	r[router.Key(base.Get, categoryendpoints.GetAllCategories)] = authz.Authenticated
	r[router.Key(base.Get, categoryendpoints.GetCategoryByID)] = authz.Authenticated
	r[router.Key(base.Post, categoryendpoints.CreateCategory)] = authz.Authenticated
	r[router.Key(base.Patch, categoryendpoints.PatchCategoryByID)] = authz.Authenticated
	r[router.Key(base.Delete, categoryendpoints.DeleteCategoryByID)] = authz.Authenticated
}

// Applications are discovered, not authored: the notifier creates them and
// users delete through discovery's reset, which also purges its Redis state.
func addApplications(r map[string]authz.Requirement) {
	r[router.Key(base.Post, applicationendpoints.CreateApplication)] = authz.Internal
	r[router.Key(base.Get, applicationendpoints.GetAllApplications)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetApplicationByName)] = authz.Read(roledata.ScopeApplications)
	r[router.Key(base.Get, applicationendpoints.GetRollbacks)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks,
	)
	r[router.Key(base.Get, applicationendpoints.GetRollback)] = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsRollbacks,
	)
	r[router.Key(base.Patch, applicationendpoints.PatchApplicationByName)] = authz.Denyable(
		authz.Write(roledata.ScopeApplications), roledata.ActionEditApplication,
	)
	r[router.Key(base.Delete, applicationendpoints.DeleteApplicationByName)] = authz.Internal
}

// Snapshots are taken of applications, so reads follow the applications scope;
// only discovery writes or discards them.
func addSnapshots(r map[string]authz.Requirement) {
	r[router.Key(base.Post, snapshotendpoints.CreateSnapshot)] = authz.Internal
	r[router.Key(base.Get, snapshotendpoints.GetSnapshot)] = snapshotView
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotInfos)] = snapshotView
	r[router.Key(base.Get, snapshotendpoints.GetSnapshotManifest)] = snapshotManifestView
	r[router.Key(base.Delete, snapshotendpoints.DeleteSnapshot)] = authz.Internal
}

// The snapshot body carries the manifests and the manifest is part of the
// snapshot, so GuardSnapshotManifestView and GuardSnapshotView hold each GET
// route to the other's rule too.
var (
	snapshotView = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationsSnapshots,
	)
	snapshotManifestView = authz.Denyable(
		authz.Read(roledata.ScopeApplications), roledata.ActionViewApplicationSnapshotManifest,
	)
)

// Reports are written by discovery at a plan boundary; users read them under
// the plan scope, and listing and downloading can each be withheld.
func addReports(r map[string]authz.Requirement) {
	r[router.Key(base.Post, reportsendpoints.CreatePlanReport)] = authz.Internal
	r[router.Key(base.Update, reportsendpoints.PutPlanReportLedger)] = authz.Internal
	r[router.Key(base.Get, reportsendpoints.GetPlanReportLedger)] = authz.Internal
	r[router.Key(base.Get, reportsendpoints.ListPlanReports)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
	)
	r[router.Key(base.Get, reportsendpoints.ListReports)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlanReports,
	)
	r[router.Key(base.Get, reportsendpoints.DownloadPlanReport)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionDownloadProtectionPlanReport,
	)
}

// Their own feature: managing an application does not imply managing its plans.
func addPlans(r map[string]authz.Requirement) {
	r[router.Key(base.Get, plansendpoints.ListProtectionPlans)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans,
	)
	r[router.Key(base.Get, plansendpoints.GetProtectionPlanByID)] = authz.Denyable(
		authz.Read(roledata.ScopeProtectionPlans), roledata.ActionViewProtectionPlans,
	)
	r[router.Key(base.Post, plansendpoints.CreateProtectionPlan)] = authz.Denyable(
		authz.Write(roledata.ScopeProtectionPlans), roledata.ActionCreateProtectionPlan,
	)
	// Users edit and delete through discovery, which owns the lifecycle and
	// removes the deployed policies first.
	r[router.Key(base.Patch, plansendpoints.PatchProtectionPlanByID)] = authz.Internal
	r[router.Key(base.Delete, plansendpoints.DeleteProtectionPlanByID)] = authz.Internal
}

// Every user reaches their own, whatever scopes their roles carry.
func addNotifications(r map[string]authz.Requirement) {
	r[router.Key(base.Get, notificationsendpoints.List)] = authz.Authenticated
	r[router.Key(base.Post, notificationsendpoints.MarkRead)] = authz.Authenticated
	r[router.Key(base.Post, notificationsendpoints.MarkAllRead)] = authz.Authenticated
	r[router.Key(base.Delete, notificationsendpoints.Clear)] = authz.Authenticated
	r[router.Key(base.Delete, notificationsendpoints.Delete)] = authz.Authenticated

	// Emitted by peer services reacting to cluster events, not by users.
	r[router.Key(base.Post, notificationsendpoints.Emit)] = authz.Internal
}

// Its parts are not governed alike, so GuardConfigPatch checks per field.
func addConfig(r map[string]authz.Requirement) {
	r[router.Key(base.Get, configendpoints.GetConfig)] = authz.Read(roledata.ScopeSettings)
	r[router.Key(base.Patch, configendpoints.PatchConfig)] = authz.Authenticated
}

// Finalizer edits decide whether a resource can be deleted at all, so only
// auth's cleanup cascade makes them.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Update, cleanupendpoints.AddFinalizer)] = authz.Internal
	r[router.Key(base.Delete, cleanupendpoints.RemoveFinalizer)] = authz.Internal
	r[router.Key(base.Get, cleanupendpoints.GetCleanupViewByID)] = authz.Read(roledata.ScopeSettings)
	r[router.Key(base.Get, cleanupendpoints.ListCleanupViews)] = authz.Read(roledata.ScopeSettings)
}

// Not administrative, so no scope; the guards narrow these to the caller.
func addSessions(r map[string]authz.Requirement) {
	r[router.Key(base.Get, authendpoints.GetAllSessionsByUser)] = authz.Authenticated
	r[router.Key(base.Get, authendpoints.GetSelfSession)] = authz.Authenticated
	r[router.Key(base.Delete, authendpoints.DeleteSelfSession)] = authz.Authenticated
	r[router.Key(base.Delete, authendpoints.DeleteSessionByName)] = authz.Authenticated

	r[router.Key(base.Post, authendpoints.CreateSessionByUser)] = authz.Internal
	r[router.Key(base.Patch, authendpoints.PatchSelfSession)] = authz.Internal
}

// Reached via the passkey proxy, during a ceremony that has no session yet.
func addPasskeys(r map[string]authz.Requirement) {
	r[router.Key(base.Post, authendpoints.CreateInternalPasskeyByUser)] = authz.Internal
	r[router.Key(base.Get, authendpoints.GetAllInternalPasskeysByUser)] = authz.Internal
	r[router.Key(base.Get, authendpoints.GetInternalPasskeyByUserAndCredentialID)] = authz.Internal
	r[router.Key(base.Patch, authendpoints.PatchInternalPasskeyByUserAndCredentialID)] = authz.Internal
	r[router.Key(base.Delete, authendpoints.DeleteInternalPasskeyByUserAndCredentialID)] = authz.Internal
}
