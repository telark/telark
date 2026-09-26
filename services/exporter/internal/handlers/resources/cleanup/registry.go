package cleanup

import (
	metadatabase "github.com/telark/data/metadata/base"
	metadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/data/resources/finalizers"
	"github.com/telark/exporter/internal/constants"
)

type resourceTarget struct {
	Metadata metadatabase.Metadata
	RefKeys  []string
}

var registry = map[string]resourceTarget{
	finalizers.ResourceTypeUsers: {
		Metadata: metadata.UserMetadata,
		RefKeys:  []string{constants.FieldRoleRefs, constants.FieldGroupRefs},
	},
	finalizers.ResourceTypeGroups: {
		Metadata: metadata.GroupMetadata,
		RefKeys:  []string{constants.FieldUserRefs, constants.FieldRoleRefs},
	},
	finalizers.ResourceTypeRoles: {
		Metadata: metadata.AccessRoleMetadata,
		RefKeys:  nil,
	},
}

func lookupTarget(resourceType string) (resourceTarget, bool) {
	target, ok := registry[resourceType]
	return target, ok
}
