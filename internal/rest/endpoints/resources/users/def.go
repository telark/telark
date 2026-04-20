package users

import "github.com/plsyro/rest/base"

const (
	CreateUser        base.Endpoint = "resources/users/create"
	GetAllUsers       base.Endpoint = "resources/users/get"
	GetUserByID       base.Endpoint = "resources/users/findbyid/{id}/get"
	GetUserByUsername base.Endpoint = "resources/users/findbyusername/{username}/get"
	GetUserByEmail    base.Endpoint = "resources/users/findbyemail/{email}/get"
	PatchUserByID     base.Endpoint = "resources/users/{id}/patch"
	DeleteUserByID    base.Endpoint = "resources/users/{id}/delete"
)
