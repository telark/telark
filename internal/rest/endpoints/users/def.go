package users

import "github.com/telark/telark/internal/rest/base"

const (
	CreateUser     base.Endpoint = "users"
	GetAllUsers    base.Endpoint = "users"
	GetUserNames   base.Endpoint = "users/names"
	GetUserByID    base.Endpoint = "users/{id}"
	PatchUserByID  base.Endpoint = "users/{id}"
	DeleteUserByID base.Endpoint = "users/{id}"

	// Service-only lookups: a distinct path keeps them off the list route's requirement
	GetUserByUsername base.Endpoint = "internal/users/by-username/{username}"
	GetUserByEmail    base.Endpoint = "internal/users/by-email/{email}"
	GetUserByIdentity base.Endpoint = "internal/users/by-identity"
)
