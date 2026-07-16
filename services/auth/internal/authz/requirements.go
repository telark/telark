package authz

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/rest/base"
	autheps "github.com/telark/rest/endpoints/auth"
	statuseps "github.com/telark/rest/endpoints/status"
	"github.com/telark/rest/router"
	"github.com/telark/x-ware/authz"
)

func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addLogin(requirements)
	addSelfService(requirements)
	addCleanup(requirements)

	return requirements
}

func addStatus(r map[string]authz.Requirement) {
	r[router.Key(base.Get, statuseps.HealthCheck)] = authz.Public
	r[router.Key(base.Get, statuseps.ReadinessCheck)] = authz.Public
	r[router.Key(base.Get, statuseps.LivenessCheck)] = authz.Public
}

// The login surface cannot require a session: it is what produces one. Each of
// these proves identity by its own means, a WebAuthn assertion or an OIDC token.
func addLogin(r map[string]authz.Requirement) {
	r[router.Key(base.Post, autheps.StartLogin)] = authz.Public
	r[router.Key(base.Post, autheps.FinishLogin)] = authz.Public
	r[router.Key(base.Post, autheps.StartRegister)] = authz.Public
	r[router.Key(base.Get, autheps.Config)] = authz.Public
	r[router.Key(base.Post, autheps.OIDCGoogleCallback)] = authz.Public
	r[router.Key(base.Post, autheps.OIDCGoogleNonce)] = authz.Public

	// Logout destroys the session named by the token the caller already holds,
	// so it is self-authorizing and must keep working for an expired session.
	r[router.Key(base.Post, autheps.Logout)] = authz.Public
}

// Routes acting only on the caller's own record. The handlers take the user
// from the validated session, never from the request.
func addSelfService(r map[string]authz.Requirement) {
	r[router.Key(base.Get, autheps.GetMyPermissions)] = authz.Authenticated
	r[router.Key(base.Get, autheps.GetAllPasskeysByUserViaProxy)] = authz.Authenticated
	r[router.Key(base.Post, autheps.CreatePasskeyByUserViaProxy)] = authz.Authenticated
	r[router.Key(base.Get, autheps.GetPasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
	r[router.Key(base.Patch, autheps.PatchPasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
	r[router.Key(base.Delete, autheps.DeletePasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
}

// Cascading deletes of a principal and everything attached to it.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Delete, autheps.DeleteUserCleanup)] = authz.Own(roledata.ScopeUsers)
	r[router.Key(base.Delete, autheps.DeleteGroupCleanup)] = authz.Own(roledata.ScopeGroups)
	r[router.Key(base.Delete, autheps.DeleteRoleCleanup)] = authz.Own(roledata.ScopeRoles)
}
