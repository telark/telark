package accessroles

import "github.com/telark/telark/internal/rest/base"

const (
	CreateAccessRole     base.Endpoint = "accessroles"
	GetAllAccessRoles    base.Endpoint = "accessroles"
	GetAccessRoleByID    base.Endpoint = "accessroles/{id}"
	PatchAccessRoleByID  base.Endpoint = "accessroles/{id}"
	DeleteAccessRoleByID base.Endpoint = "accessroles/{id}"
)
