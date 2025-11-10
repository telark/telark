package auth

import "github.com/plsyro/rest/base"

const (
	// Login & logout
	StartLogin  base.Endpoint = "auth/login/start"
	FinishLogin base.Endpoint = "auth/login/finish"
	Logout      base.Endpoint = "auth/logout"

	// Passkeys
	GetAllPasskeysByUser base.Endpoint = "auth/passkeys/{userId}/get"
	GetPasskeyByUser     base.Endpoint = "auth/passkeys/{userId}/{id}/get"
	CreatePasskeyByUser  base.Endpoint = "auth/passkeys/{userId}/create"
	PatchPasskeyByUser   base.Endpoint = "auth/passkeys/{userId}/{id}/patch"
	DeletePasskeyByUser  base.Endpoint = "auth/passkeys/{userId}/{id}/delete"

	// Sessions
	CreateSessionByUser  base.Endpoint = "auth/sessions/{userId}/create"
	GetAllSessionsByUser base.Endpoint = "auth/sessions/{userId}/get"
	GetSessionByToken    base.Endpoint = "auth/sessions/{token}/get"
	DeleteSessionByToken base.Endpoint = "auth/sessions/{token}/delete"
	PatchSessionByToken  base.Endpoint = "auth/sessions/{token}/patch"

	// Challenges
	CreateChallengeByUser base.Endpoint = "auth/challenges/{userId}/create"
	GetChallengeByUser    base.Endpoint = "auth/challenges/{userId}/get"
	DeleteChallengeByUser base.Endpoint = "auth/challenges/{userId}/delete"
)
