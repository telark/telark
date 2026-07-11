package routes

import (
	authhandler "github.com/telark/auth/internal/handlers/auth"
	authorisationhandler "github.com/telark/auth/internal/handlers/authorisation"
	cleanuphandler "github.com/telark/auth/internal/handlers/cleanup"
	confighandler "github.com/telark/auth/internal/handlers/config"
	oidchandler "github.com/telark/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/auth/internal/handlers/passkey"
	statushandler "github.com/telark/auth/internal/handlers/status"
	"github.com/telark/rest/base"
	autheps "github.com/telark/rest/endpoints/auth"
	statuseps "github.com/telark/rest/endpoints/status"
	"github.com/telark/rest/router"
)

var Routes = []router.Route{
	// Authentication routes
	router.CreateRoute(base.Post, autheps.StartLogin, authhandler.LoginStart),
	router.CreateRoute(base.Post, autheps.FinishLogin, authhandler.LoginFinish),
	router.CreateRoute(base.Post, autheps.Logout, authhandler.Logout),

	// Authorisation
	router.CreateRoute(base.Get, autheps.GetMyPermissions, authorisationhandler.GetPermissions),

	// Registration routes
	router.CreateRoute(base.Post, autheps.StartRegister, passkeyhandler.RegisterStart),

	// Public bootstrap config (unauthenticated)
	router.CreateRoute(base.Get, autheps.Config, confighandler.GetConfig),

	// Passkey management routes (proxy)
	router.CreateRoute(base.Get, autheps.GetAllPasskeysByUserViaProxy, passkeyhandler.GetPasskeys),
	router.CreateRoute(base.Post, autheps.CreatePasskeyByUserViaProxy, passkeyhandler.CreatePasskey),
	router.CreateRoute(base.Get, autheps.GetPasskeyByUserAndCredentialIDViaProxy, passkeyhandler.GetSinglePasskey),
	router.CreateRoute(base.Patch, autheps.PatchPasskeyByUserAndCredentialIDViaProxy, passkeyhandler.UpdatePasskey),
	router.CreateRoute(base.Delete, autheps.DeletePasskeyByUserAndCredentialIDViaProxy, passkeyhandler.DeletePasskey),

	// OIDC routes
	router.CreateRoute(base.Post, autheps.OIDCGoogleCallback, oidchandler.GoogleCallback),
	router.CreateRoute(base.Post, autheps.OIDCGoogleNonce, oidchandler.GetNonce),

	// Status routes
	router.CreateRoute(base.Get, statuseps.HealthCheck, statushandler.HealthHandler),
	router.CreateRoute(base.Get, statuseps.ReadinessCheck, statushandler.ReadinessHandler),
	router.CreateRoute(base.Get, statuseps.LivenessCheck, statushandler.LivenessHandler),

	// Cleanup (async business delete)
	router.CreateRoute(base.Delete, autheps.DeleteUserCleanup, cleanuphandler.DeleteUser),
	router.CreateRoute(base.Delete, autheps.DeleteGroupCleanup, cleanuphandler.DeleteGroup),
	router.CreateRoute(base.Delete, autheps.DeleteRoleCleanup, cleanuphandler.DeleteRole),
}
