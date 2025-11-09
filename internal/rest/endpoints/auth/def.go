package auth

import "github.com/plsyro/rest/base"

const (
	StartLogin     base.Endpoint = "auth/login/start"
	FinishLogin    base.Endpoint = "auth/login/finish"
	Logout         base.Endpoint = "auth/logout"
	GetAllPasskeys base.Endpoint = "auth/passkeys/get"
	GetPasskey     base.Endpoint = "auth/passkeys/{id}/get"
	CreatePasskey  base.Endpoint = "auth/passkeys/create"
	PatchPasskey   base.Endpoint = "auth/passkeys/{id}/patch"
	DeletePasskey  base.Endpoint = "auth/passkeys/{id}/delete"
)
