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
	CreateInternalPasskeyByUser                base.Endpoint = "internal/auth/passkeys"
	GetAllInternalPasskeysByUser               base.Endpoint = "internal/auth/passkeys"
	GetInternalPasskeyByUserAndCredentialID    base.Endpoint = "internal/auth/passkeys/{credentialId}"
	PatchInternalPasskeyByUserAndCredentialID  base.Endpoint = "internal/auth/passkeys/{credentialId}"
	DeleteInternalPasskeyByUserAndCredentialID base.Endpoint = "internal/auth/passkeys/{credentialId}"

	// Served by the auth service, proxying the internal passkey endpoints above
	GetAllPasskeysByUserViaProxy               base.Endpoint = "auth/passkeys"
	CreatePasskeyByUserViaProxy                base.Endpoint = "auth/passkeys"
	GetPasskeyByUserAndCredentialIDViaProxy    base.Endpoint = "auth/passkeys/{credentialId}"
	PatchPasskeyByUserAndCredentialIDViaProxy  base.Endpoint = "auth/passkeys/{credentialId}"
	DeletePasskeyByUserAndCredentialIDViaProxy base.Endpoint = "auth/passkeys/{credentialId}"
	CreatePasskeyEnrollLink                    base.Endpoint = "auth/passkeys/enroll-link"

	// The self session is the one the X-Session-Token header names; the token never enters a path
	CreateSessionByUser  base.Endpoint = "internal/auth/users/{userId}/sessions"
	GetAllSessionsByUser base.Endpoint = "auth/sessions"
	GetSelfSession       base.Endpoint = "auth/sessions/self"
	DeleteSelfSession    base.Endpoint = "auth/sessions/self"
	PatchSelfSession     base.Endpoint = "auth/sessions/self"
	// Another session of the caller's own account, addressed by its session-<sha256> name.
	DeleteSessionByName base.Endpoint = "auth/sessions/{name}"

	GetMyPermissions base.Endpoint = "auth/permissions"

	OIDCGoogleCallback base.Endpoint = "auth/oidc/google/callback"
	OIDCGoogleNonce    base.Endpoint = "auth/oidc/google/nonce"
	OIDCConfig         base.Endpoint = "auth/oidc/config"

	// Async business delete; the finalizer-backed cleanup runs afterwards
	DeleteUserCleanup       base.Endpoint = "auth/users/{id}"
	DeleteGroupCleanup      base.Endpoint = "auth/groups/{id}"
	DeleteAccessRoleCleanup base.Endpoint = "auth/accessroles/{id}"

	QuerySessionUser = "user"
)
