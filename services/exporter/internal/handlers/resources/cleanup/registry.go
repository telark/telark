package cleanup

import (
	metadatabase "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/data/resources/finalizers"
	"github.com/telark/exporter/internal/constants"
)

type resourceTarget struct {
	Metadata metadatabase.Metadata
	RefKeys  []string
}

var registry = map[string]resourceTarget{
	finalizers.ResourceTypeUsers: {
		Metadata: metadata.UserAsResourceMetadata,
		RefKeys:  []string{constants.FieldAssignedRolesIDs, constants.FieldAssignedGroupsIDs},
	},
	finalizers.ResourceTypeGroups: {
		Metadata: metadata.GroupAsResourceMetadata,
		RefKeys:  []string{constants.FieldAssignedUsersIDs, constants.FieldAssignedRolesIDs},
	},
	finalizers.ResourceTypeRoles: {
		Metadata: metadata.RoleAsResourceMetadata,
		RefKeys:  nil,
	},
}

func lookupTarget(resourceType string) (resourceTarget, bool) {
	target, ok := registry[resourceType]
	return target, ok
}
