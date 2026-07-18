package auth

import "github.com/telark/rest/base"

const (
	// Login & logout
	StartLogin    base.Endpoint = "auth/login/start"
	FinishLogin   base.Endpoint = "auth/login/finish"
	StartRegister base.Endpoint = "auth/register/start"
	Logout        base.Endpoint = "auth/logout"

	// Public bootstrap config (no authentication)
	Config base.Endpoint = "auth/config"

	// Internal Passkeys EPs used only exporter-service
	CreateInternalPasskeyByUser                base.Endpoint = "auth/passkeys/internal/create"
	GetAllInternalPasskeysByUser               base.Endpoint = "auth/passkeys/internal/get"
	GetInternalPasskeyByUserAndCredentialID    base.Endpoint = "auth/passkeys/internal/single/get"
	PatchInternalPasskeyByUserAndCredentialID  base.Endpoint = "auth/passkeys/internal/patch"
	DeleteInternalPasskeyByUserAndCredentialID base.Endpoint = "auth/passkeys/internal/delete"

	// used by auth-service as proxy to exporter-service internal passkeys EPs
	GetAllPasskeysByUserViaProxy               base.Endpoint = "auth/passkeys/proxy/get"
	CreatePasskeyByUserViaProxy                base.Endpoint = "auth/passkeys/proxy/create"
	GetPasskeyByUserAndCredentialIDViaProxy    base.Endpoint = "auth/passkeys/proxy/single/get"
	PatchPasskeyByUserAndCredentialIDViaProxy  base.Endpoint = "auth/passkeys/proxy/patch"
	DeletePasskeyByUserAndCredentialIDViaProxy base.Endpoint = "auth/passkeys/proxy/delete"

	// Sessions
	CreateSessionByUser  base.Endpoint = "auth/sessions/{userId}/create"
	GetAllSessionsByUser base.Endpoint = "auth/sessions/{userId}/get"
	GetSessionByToken    base.Endpoint = "auth/sessions/tokens/{token}/get"
	DeleteSessionByToken base.Endpoint = "auth/sessions/tokens/{token}/delete"
	PatchSessionByToken  base.Endpoint = "auth/sessions/tokens/{token}/patch"

	// Authorisation
	GetMyPermissions base.Endpoint = "auth/permissions"

	// OIDC
	OIDCGoogleCallback base.Endpoint = "auth/oidc/google/callback"
	OIDCGoogleNonce    base.Endpoint = "auth/oidc/google/nonce"
	OIDCConfig         base.Endpoint = "auth/oidc/config"

	// Cleanup (async business delete; finalizer-backed cleanup runs after)
	DeleteUserCleanup  base.Endpoint = "auth/users/{id}/cleanup"
	DeleteGroupCleanup base.Endpoint = "auth/groups/{id}/cleanup"
	DeleteRoleCleanup  base.Endpoint = "auth/roles/{id}/cleanup"
)
