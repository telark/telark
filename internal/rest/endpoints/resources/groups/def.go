package groups

import "github.com/telark/rest/base"

const (
	CreateGroup     base.Endpoint = "resources/groups/create"
	GetAllGroups    base.Endpoint = "resources/groups/get"
	GetGroupByID    base.Endpoint = "resources/groups/{id}/get"
	PatchGroupByID  base.Endpoint = "resources/groups/{id}/patch"
	DeleteGroupByID base.Endpoint = "resources/groups/{id}/delete"
)
