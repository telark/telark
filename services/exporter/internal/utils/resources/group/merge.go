package group

import (
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
)

func MergeGroupAndPreparePatchBody(existingGroup, newGroup *groupdata.GroupAsResource, body map[string]any) *groupdata.GroupAsResource {
	mergedGroup := *existingGroup

	if newGroup.Name != constants.EmptyString {
		mergedGroup.Name = newGroup.Name
	}
	if newGroup.Description != constants.EmptyString {
		mergedGroup.Description = newGroup.Description
	}
	if newGroup.CategoryID != constants.EmptyString {
		mergedGroup.CategoryID = newGroup.CategoryID
	}

	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldAssignedUsersIDs,
		newGroup.AssignedUsersIDs,
		&mergedGroup.AssignedUsersIDs,
	)
	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldAssignedRolesIDs,
		newGroup.AssignedRolesIDs,
		&mergedGroup.AssignedRolesIDs,
	)

	if newGroup.CreatedBy != nil {
		mergedGroup.CreatedBy = newGroup.CreatedBy
		body["createdBy"] = *newGroup.CreatedBy
	}
	if newGroup.LastUpdatedBy != nil {
		mergedGroup.LastUpdatedBy = newGroup.LastUpdatedBy
		body["lastUpdatedBy"] = *newGroup.LastUpdatedBy
	}

	return &mergedGroup
}
