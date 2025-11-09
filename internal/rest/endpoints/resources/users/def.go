package users

import "github.com/plsyro/rest/base"

const (
	CreateUser  base.Endpoint = "resources/users/create"
	GetAllUsers base.Endpoint = "resources/users/get"
	GetUser     base.Endpoint = "resources/users/{name}/get"
	PatchUser   base.Endpoint = "resources/users/{name}/patch"
	DeleteUser  base.Endpoint = "resources/users/{name}/delete"
)
