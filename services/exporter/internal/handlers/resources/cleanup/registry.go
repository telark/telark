package cleanup

import (
	metadatabase "github.com/telark/telark/internal/data/metadata/base"
	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"github.com/telark/telark/services/exporter/internal/constants"
)

type resourceTarget struct {
	Metadata  metadatabase.Metadata
	RefKeys   []string
	CacheType string
}

var registry = map[string]resourceTarget{
	finalizers.ResourceTypeUsers: {
		Metadata:  metadata.UserMetadata,
		RefKeys:   []string{constants.FieldRoleRefs, constants.FieldGroupRefs},
		CacheType: constants.ResourceUser,
	},
	finalizers.ResourceTypeGroups: {
		Metadata:  metadata.GroupMetadata,
		RefKeys:   []string{constants.FieldUserRefs, constants.FieldRoleRefs},
		CacheType: constants.ResourceGroup,
	},
	finalizers.ResourceTypeRoles: {
		Metadata:  metadata.AccessRoleMetadata,
		RefKeys:   nil,
		CacheType: constants.ResourceRole,
	},
}

func lookupTarget(resourceType string) (resourceTarget, bool) {
	target, ok := registry[resourceType]
	return target, ok
}
