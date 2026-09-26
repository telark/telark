package groups

import "github.com/telark/rest/base"

const (
	CreateGroup     base.Endpoint = "groups"
	GetAllGroups    base.Endpoint = "groups"
	GetGroupByID    base.Endpoint = "groups/{id}"
	PatchGroupByID  base.Endpoint = "groups/{id}"
	DeleteGroupByID base.Endpoint = "groups/{id}"
)
