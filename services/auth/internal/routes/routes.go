package routes

import (
	"github.com/telark/telark/internal/rest/base"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	statuseps "github.com/telark/telark/internal/rest/endpoints/status"
	"github.com/telark/telark/internal/rest/router"
	authhandler "github.com/telark/telark/services/auth/internal/handlers/auth"
	authorisationhandler "github.com/telark/telark/services/auth/internal/handlers/authorisation"
	cleanuphandler "github.com/telark/telark/services/auth/internal/handlers/cleanup"
	confighandler "github.com/telark/telark/services/auth/internal/handlers/config"
	oidchandler "github.com/telark/telark/services/auth/internal/handlers/oidc"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	statushandler "github.com/telark/telark/services/auth/internal/handlers/status"
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
	router.CreateRoute(base.Post, autheps.CreatePasskeyEnrollLink, passkeyhandler.CreateEnrollLink),
	router.CreateRoute(base.Get, autheps.GetPasskeyByUserAndCredentialIDViaProxy, passkeyhandler.GetSinglePasskey),
	router.CreateRoute(base.Patch, autheps.PatchPasskeyByUserAndCredentialIDViaProxy, passkeyhandler.UpdatePasskey),
	router.CreateRoute(base.Delete, autheps.DeletePasskeyByUserAndCredentialIDViaProxy, passkeyhandler.DeletePasskey),

	// OIDC routes
	router.CreateRoute(base.Post, autheps.OIDCGoogleCallback, oidchandler.GoogleCallback),
	router.CreateRoute(base.Post, autheps.OIDCGoogleNonce, oidchandler.GetNonce),
	router.CreateRoute(base.Patch, autheps.OIDCConfig, oidchandler.SetConfig),

	// Status routes
	router.CreateRoute(base.Get, statuseps.HealthCheck, statushandler.ProbeHandler),
	router.CreateRoute(base.Get, statuseps.ReadinessCheck, statushandler.ProbeHandler),
	router.CreateRoute(base.Get, statuseps.LivenessCheck, statushandler.ProbeHandler),

	// Cleanup (async business delete)
	router.CreateRoute(base.Delete, autheps.DeleteUserCleanup, cleanuphandler.DeleteUser),
	router.CreateRoute(base.Delete, autheps.DeleteGroupCleanup, cleanuphandler.DeleteGroup),
	router.CreateRoute(base.Delete, autheps.DeleteAccessRoleCleanup, cleanuphandler.DeleteAccessRole),
}
