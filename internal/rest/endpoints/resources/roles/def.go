package roles

import "github.com/plsyro/rest/base"

const (
	CreateRole        base.Endpoint = "resources/roles/create"
	GetAllRoles       base.Endpoint = "resources/roles/get"
	GetRoleByID       base.Endpoint = "resources/roles/{id}/get"
	GetRoleByUserID   base.Endpoint = "resources/roles/findbyuserid/{userId}/get"
	GetRolesByUserID  base.Endpoint = "resources/roles/findbyuserid/{userId}/list"
	GetRoleByGroupID  base.Endpoint = "resources/roles/findbygroupid/{groupId}/get"
	GetRolesByGroupID base.Endpoint = "resources/roles/findbygroupid/{groupId}/list"
	PatchRoleByID     base.Endpoint = "resources/roles/{id}/patch"
	DeleteRoleByID    base.Endpoint = "resources/roles/{id}/delete"
)
