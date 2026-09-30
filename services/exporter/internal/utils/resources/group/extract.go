package group

import (
	groupdata "github.com/telark/telark/internal/data/resources/group"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func ExtractGroupSpecFromRequestBody(body map[string]any) (*groupdata.Group, error) {
	group, err := sharedutils.ExtractStructFromBodyIgnoringID[groupdata.Group](body)
	if err != nil {
		return nil, err
	}

	group.UserRefs = resourcesshared.DedupeIDs(group.UserRefs)
	group.RoleRefs = resourcesshared.DedupeIDs(group.RoleRefs)
	return group, nil
}
