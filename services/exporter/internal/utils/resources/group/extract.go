package group

import (
	groupdata "github.com/telark/data/resources/group"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
)

func ExtractGroupSpecFromRequestBody(body map[string]any) (*groupdata.GroupAsResource, error) {
	group, err := sharedutils.ExtractStructFromBodyIgnoringID[groupdata.GroupAsResource](body)
	if err != nil {
		return nil, err
	}

	resourcesshared.InitializeIDs(&group.AssignedUsersIDs)
	resourcesshared.InitializeIDs(&group.AssignedRolesIDs)
	return group, nil
}
