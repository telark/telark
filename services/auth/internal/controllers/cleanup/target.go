package cleanup

import (
	"github.com/telark/auth/internal/constants"
	"github.com/telark/data/resources/finalizers"
)

type refSpec struct {
	ResourceType string
	ArrayField   string
}

var targetRefs = map[string][]refSpec{
	finalizers.ResourceTypeUsers: {
		{ResourceType: finalizers.ResourceTypeGroups, ArrayField: constants.SpecFieldUserRefs},
	},
	finalizers.ResourceTypeGroups: {
		{ResourceType: finalizers.ResourceTypeUsers, ArrayField: constants.SpecFieldGroupRefs},
	},
	finalizers.ResourceTypeRoles: {
		{ResourceType: finalizers.ResourceTypeUsers, ArrayField: constants.SpecFieldRoleRefs},
		{ResourceType: finalizers.ResourceTypeGroups, ArrayField: constants.SpecFieldRoleRefs},
	},
}

var targetPurges = map[string]PurgeFn{
	finalizers.ResourceTypeUsers: purgeUserSessions,
}

func DefaultTargets() map[string]Target {
	out := make(map[string]Target, len(targetRefs))
	for _, resourceType := range RegisteredResourceTypes() {
		out[resourceType] = buildTarget(resourceType)
	}
	return out
}

func buildTarget(resourceType string) Target {
	ops, _ := GetResourceOps(resourceType)
	refs := targetRefs[resourceType]
	backRefs := make([]BackRef, constants.DefaultInitValue, len(refs))
	for _, r := range refs {
		refOps, ok := GetResourceOps(r.ResourceType)
		if !ok {
			continue
		}
		backRefs = append(backRefs, BackRef{
			ResourceType: r.ResourceType,
			ArrayField:   r.ArrayField,
			List:         refOps.List,
			Patch:        refOps.Patch,
		})
	}
	return Target{
		ResourceType:    resourceType,
		Finalizer:       ops.Finalizer,
		Purge:           targetPurges[resourceType],
		BackRefs:        backRefs,
		RemoveFinalizer: ops.RemoveFinalizer,
	}
}
