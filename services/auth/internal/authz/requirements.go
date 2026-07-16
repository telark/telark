package authz

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/rest/base"
	autheps "github.com/telark/rest/endpoints/auth"
	statuseps "github.com/telark/rest/endpoints/status"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/authz"
)

var (
	public        = authz.Requirement{Access: authz.AccessPublic}
	authenticated = authz.Requirement{Access: authz.AccessAuthenticated}
)

func remove(scope string) authz.Requirement {
	return authz.Requirement{Scope: scope, MinLevel: roledata.PermissionLevelOwner}
}

func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addLogin(requirements)
	addSelfService(requirements)
	addCleanup(requirements)

	return requirements
}

func addStatus(r map[string]authz.Requirement) {
	r[router.Key(base.Get, statuseps.HealthCheck)] = public
	r[router.Key(base.Get, statuseps.ReadinessCheck)] = public
	r[router.Key(base.Get, statuseps.LivenessCheck)] = public
}

// The login surface cannot require a session: it is what produces one. Each of
// these proves identity by its own means, a WebAuthn assertion or an OIDC token.
func addLogin(r map[string]authz.Requirement) {
	r[router.Key(base.Post, autheps.StartLogin)] = public
	r[router.Key(base.Post, autheps.FinishLogin)] = public
	r[router.Key(base.Post, autheps.StartRegister)] = public
	r[router.Key(base.Get, autheps.Config)] = public
	r[router.Key(base.Post, autheps.OIDCGoogleCallback)] = public
	r[router.Key(base.Post, autheps.OIDCGoogleNonce)] = public

	// Logout destroys the session named by the token the caller already holds,
	// so it is self-authorizing and must keep working for an expired session.
	r[router.Key(base.Post, autheps.Logout)] = public
}

// Routes acting only on the caller's own record. The handlers take the user
// from the validated session, never from the request.
func addSelfService(r map[string]authz.Requirement) {
	r[router.Key(base.Get, autheps.GetMyPermissions)] = authenticated
	r[router.Key(base.Get, autheps.GetAllPasskeysByUserViaProxy)] = authenticated
	r[router.Key(base.Post, autheps.CreatePasskeyByUserViaProxy)] = authenticated
	r[router.Key(base.Get, autheps.GetPasskeyByUserAndCredentialIDViaProxy)] = authenticated
	r[router.Key(base.Patch, autheps.PatchPasskeyByUserAndCredentialIDViaProxy)] = authenticated
	r[router.Key(base.Delete, autheps.DeletePasskeyByUserAndCredentialIDViaProxy)] = authenticated
}

// Cascading deletes of a principal and everything attached to it.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Delete, autheps.DeleteUserCleanup)] = remove(roledata.ScopeUsers)
	r[router.Key(base.Delete, autheps.DeleteGroupCleanup)] = remove(roledata.ScopeGroups)
	r[router.Key(base.Delete, autheps.DeleteRoleCleanup)] = remove(roledata.ScopeRoles)
}
