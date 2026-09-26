package auth

import "github.com/telark/rest/base"

const (
	StartLogin    base.Endpoint = "auth/login/start"
	FinishLogin   base.Endpoint = "auth/login/finish"
	StartRegister base.Endpoint = "auth/register/start"
	Logout        base.Endpoint = "auth/logout"

	// Public bootstrap config (no authentication)
	Config base.Endpoint = "auth/config"

	// Served by the exporter only
	CreateInternalPasskeyByUser                base.Endpoint = "auth/passkeys/internal/create"
	GetAllInternalPasskeysByUser               base.Endpoint = "auth/passkeys/internal/get"
	GetInternalPasskeyByUserAndCredentialID    base.Endpoint = "auth/passkeys/internal/single/get"
	PatchInternalPasskeyByUserAndCredentialID  base.Endpoint = "auth/passkeys/internal/patch"
	DeleteInternalPasskeyByUserAndCredentialID base.Endpoint = "auth/passkeys/internal/delete"

	// Served by the auth service, proxying the internal passkey endpoints above
	GetAllPasskeysByUserViaProxy               base.Endpoint = "auth/passkeys/proxy/get"
	CreatePasskeyByUserViaProxy                base.Endpoint = "auth/passkeys/proxy/create"
	GetPasskeyByUserAndCredentialIDViaProxy    base.Endpoint = "auth/passkeys/proxy/single/get"
	PatchPasskeyByUserAndCredentialIDViaProxy  base.Endpoint = "auth/passkeys/proxy/patch"
	DeletePasskeyByUserAndCredentialIDViaProxy base.Endpoint = "auth/passkeys/proxy/delete"
	CreatePasskeyEnrollLink                    base.Endpoint = "auth/passkeys/enroll-link"

	CreateSessionByUser  base.Endpoint = "auth/sessions/{userId}/create"
	GetAllSessionsByUser base.Endpoint = "auth/sessions/{userId}/get"
	GetSessionByToken    base.Endpoint = "auth/sessions/tokens/{token}/get"
	DeleteSessionByToken base.Endpoint = "auth/sessions/tokens/{token}/delete"
	PatchSessionByToken  base.Endpoint = "auth/sessions/tokens/{token}/patch"

	GetMyPermissions base.Endpoint = "auth/permissions"

	OIDCGoogleCallback base.Endpoint = "auth/oidc/google/callback"
	OIDCGoogleNonce    base.Endpoint = "auth/oidc/google/nonce"
	OIDCConfig         base.Endpoint = "auth/oidc/config"

	// Async business delete; the finalizer-backed cleanup runs afterwards
	DeleteUserCleanup  base.Endpoint = "auth/users/{id}/cleanup"
	DeleteGroupCleanup base.Endpoint = "auth/groups/{id}/cleanup"
	DeleteRoleCleanup  base.Endpoint = "auth/roles/{id}/cleanup"
)
