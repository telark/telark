package auth

import "github.com/plsyro/rest/base"

const (
	// Login & logout
	StartLogin  base.Endpoint = "auth/login/start"
	FinishLogin base.Endpoint = "auth/login/finish"
	Logout      base.Endpoint = "auth/logout"

	// Passkeys
	CreatePasskeyByUser                base.Endpoint = "auth/passkeys/create"
	GetAllPasskeysByUser               base.Endpoint = "auth/passkeys/get"
	GetPasskeyByUserAndCredentialID    base.Endpoint = "auth/passkeys/single/get"
	PatchPasskeyByUserAndCredentialID  base.Endpoint = "auth/passkeys/patch"
	DeletePasskeyByUserAndCredentialID base.Endpoint = "auth/passkeys/delete"

	// Sessions
	CreateSessionByUser  base.Endpoint = "auth/sessions/{userId}/create"
	GetAllSessionsByUser base.Endpoint = "auth/sessions/{userId}/get"
	GetSessionByToken    base.Endpoint = "auth/sessions/tokens/{token}/get"
	DeleteSessionByToken base.Endpoint = "auth/sessions/tokens/{token}/delete"
	PatchSessionByToken  base.Endpoint = "auth/sessions/tokens/{token}/patch"

	// Challenges
	CreateChallengeByUser base.Endpoint = "auth/challenges/{userId}/create"
	GetChallengeByUser    base.Endpoint = "auth/challenges/{userId}/get"
	DeleteChallengeByUser base.Endpoint = "auth/challenges/{userId}/delete"
)
