package role

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
)

func ExtractRoleSpecFromRequestBody(body map[string]any) (*roledata.RoleAsResource, error) {
	// Remove priority and version from body - these are computed server-side
	delete(body, "priority")
	delete(body, "version")

	role, err := sharedutils.ExtractStructFromBodyIgnoringID[roledata.RoleAsResource](body)
	if err != nil {
		return nil, err
	}

	if role.Status == constants.EmptyString {
		role.Status = roledata.RoleStatusActive
	}

	if role.Description == constants.EmptyString {
		role.Description = constants.EmptyString
	}

	if role.CategoryID == constants.EmptyString {
		role.CategoryID = constants.EmptyString
	}

	if role.Validity == nil {
		role.Validity = &roledata.Validity{
			Type: roledata.ValidityTypePermanent,
		}
	}

	if role.Protection == nil {
		role.Protection = &roledata.Protection{
			PreventDeletion:     false,
			PreventModification: false,
			PreventScopeChanges: false,
			LockName:            false,
			LockCategory:        false,
			SoftDelete:          false,
		}
	}

	return role, nil
}
