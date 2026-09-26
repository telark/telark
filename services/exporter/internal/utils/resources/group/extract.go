package group

import (
	groupdata "github.com/telark/data/resources/group"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

func ExtractGroupSpecFromRequestBody(body map[string]any) (*groupdata.GroupAsResource, error) {
	group, err := sharedutils.ExtractStructFromBodyIgnoringID[groupdata.GroupAsResource](body)
	if err != nil {
		return nil, err
	}

	group.AssignedUsersIDs = resourcesshared.DedupeIDs(group.AssignedUsersIDs)
	group.AssignedRolesIDs = resourcesshared.DedupeIDs(group.AssignedRolesIDs)
	return group, nil
}
