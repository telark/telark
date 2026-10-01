package authz

import (
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	statuseps "github.com/telark/telark/internal/rest/endpoints/status"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/internal/x-ware/authz"
)

func Requirements() map[string]authz.Requirement {
	requirements := map[string]authz.Requirement{}

	addStatus(requirements)
	addLogin(requirements)
	addSelfService(requirements)
	addCleanup(requirements)
	addIdentityProvider(requirements)
	addEnrollLinks(requirements)

	return requirements
}

// The exporter takes these settings from auth only (Internal in its configFields), and
// each handler then requires the bootstrap account, whatever these levels grant.
func addIdentityProvider(r map[string]authz.Requirement) {
	r[router.Key(base.Patch, autheps.OIDCConfig)] = authz.Denyable(
		authz.Administer(roledata.ScopeSettings),
		roledata.ActionEditOIDCConfig,
	)
	r[router.Key(base.Patch, autheps.SelfRegistration)] = authz.Administer(roledata.ScopeSettings)
}

// The handler then caps the target's levels by the caller's and refuses hidden,
// bootstrap and, without Admin on ALL, already enrolled accounts.
func addEnrollLinks(r map[string]authz.Requirement) {
	r[router.Key(base.Post, autheps.UserEnrollLink)] = authz.Own(roledata.ScopeUsers)
	r[router.Key(base.Delete, autheps.UserEnrollLink)] = authz.Own(roledata.ScopeUsers)
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
	// Registration finish: a signed-in user adds a passkey through its session, a
	// new one completes the ceremony register/start opened, bound by its challenge.
	r[router.Key(base.Post, autheps.CreatePasskeyByUserViaProxy)] = authz.Public
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
	r[router.Key(base.Get, autheps.GetPasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
	r[router.Key(base.Patch, autheps.PatchPasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
	r[router.Key(base.Delete, autheps.DeletePasskeyByUserAndCredentialIDViaProxy)] = authz.Authenticated
	r[router.Key(base.Post, autheps.CreatePasskeyEnrollLink)] = authz.Authenticated
}

// Cascading deletes. The handler reaches the exporter with the service token, which
// its guard waves through, so the delete deny rules are enforced here or nowhere.
func addCleanup(r map[string]authz.Requirement) {
	r[router.Key(base.Delete, autheps.DeleteUserCleanup)] = authz.Denyable(
		authz.Own(roledata.ScopeUsers), roledata.ActionDeleteUser)
	r[router.Key(base.Delete, autheps.DeleteGroupCleanup)] = authz.Denyable(
		authz.Own(roledata.ScopeGroups), roledata.ActionDeleteGroup)
	r[router.Key(base.Delete, autheps.DeleteAccessRoleCleanup)] = authz.Denyable(
		authz.Own(roledata.ScopeRoles), roledata.ActionDeleteRole)
}
