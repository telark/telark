package group

import (
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
)

func MergeGroupAndPreparePatchBody(existingGroup, newGroup *groupdata.Group, body map[string]any) *groupdata.Group {
	mergedGroup := *existingGroup

	if newGroup.Name != constants.EmptyString {
		mergedGroup.Name = newGroup.Name
	}
	if newGroup.Description != constants.EmptyString {
		mergedGroup.Description = newGroup.Description
	}
	if newGroup.CategoryRef != constants.EmptyString {
		mergedGroup.CategoryRef = newGroup.CategoryRef
	}

	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldUserRefs,
		newGroup.UserRefs,
		&mergedGroup.UserRefs,
	)
	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldRoleRefs,
		newGroup.RoleRefs,
		&mergedGroup.RoleRefs,
	)

	if newGroup.CreatedBy != nil {
		mergedGroup.CreatedBy = newGroup.CreatedBy
		body[constants.FieldCreatedBy] = *newGroup.CreatedBy
	}
	if newGroup.LastUpdatedBy != nil {
		mergedGroup.LastUpdatedBy = newGroup.LastUpdatedBy
		body[constants.FieldLastUpdatedBy] = *newGroup.LastUpdatedBy
	}

	return &mergedGroup
}
