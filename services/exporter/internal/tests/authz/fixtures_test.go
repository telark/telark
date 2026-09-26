package authz

import (
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
)

const (
	roleAllAdmin    = "r-00000-0000-0009"
	roleAllAdminOff = "r-00000-0000-000c"
	roleTerminating = "r-00000-0000-000d"

	userPlain         = "u-00003-0000-0003"
	userAdmin         = "u-00004-0000-0004"
	userBootstrap     = "u-00005-0000-0005"
	userGroupAdmin    = "u-00006-0000-0006"
	userAdminOff      = "u-00007-0000-0007"
	userTerminating   = "u-00008-0000-0008"
	groupAdmin        = "ug-00001-0000-0001"
	groupPlain        = "ug-00002-0000-0002"
	groupTerminating  = "ug-00003-0000-0003"
	groupAdminGone    = "ug-00004-0000-0004"
	deletedTimestamp  = "2026-09-26T10:00:00Z"
	unknownID         = "u-ffff0-0000-0000"
	statusPhaseActive = "active"
)

func inactive(role *roledata.RoleAsResource) *roledata.RoleAsResource {
	role.Status = roledata.RoleStatusInactive
	return role
}

func terminating(role *roledata.RoleAsResource) *roledata.RoleAsResource {
	deleted := deletedTimestamp
	role.DeletionTimestamp = &deleted
	return role
}

func userHolding(id string, roleIDs, groupIDs []string) *userdata.UserAsResource {
	return &userdata.UserAsResource{
		ID:                id,
		AssignedRolesIDs:  ptrs(roleIDs),
		AssignedGroupsIDs: ptrs(groupIDs),
		Status:            userdata.UserStatus{Phase: statusPhaseActive},
	}
}

func ptrs(ids []string) []*string {
	out := make([]*string, len(ids))
	for i := range ids {
		out[i] = &ids[i]
	}
	return out
}

func fakeUsers() map[string]*userdata.UserAsResource {
	deleted := deletedTimestamp
	bootstrap := userHolding(userBootstrap, []string{roleAllAdmin}, nil)
	bootstrap.Bootstrap = true
	gone := userHolding(userTerminating, nil, nil)
	gone.DeletionTimestamp = &deleted
	return map[string]*userdata.UserAsResource{
		callerID:        userHolding(callerID, []string{roleUsersOwner}, nil),
		victimID:        userHolding(victimID, nil, nil),
		userPlain:       userHolding(userPlain, []string{roleUsersOwner}, []string{groupPlain}),
		userAdmin:       userHolding(userAdmin, []string{roleAllAdmin}, nil),
		userBootstrap:   bootstrap,
		userGroupAdmin:  userHolding(userGroupAdmin, nil, []string{groupAdmin}),
		userAdminOff:    userHolding(userAdminOff, []string{roleAllAdminOff}, []string{groupAdminGone}),
		userTerminating: gone,
	}
}

func fakeGroups() map[string]*groupdata.GroupAsResource {
	deleted := deletedTimestamp
	return map[string]*groupdata.GroupAsResource{
		groupAdmin:       {ID: groupAdmin, AssignedRolesIDs: []string{roleAllAdmin}},
		groupPlain:       {ID: groupPlain, AssignedRolesIDs: []string{roleUsersOwner}},
		groupTerminating: {ID: groupTerminating, DeletionTimestamp: &deleted},
		groupAdminGone:   {ID: groupAdminGone, AssignedRolesIDs: []string{roleAllAdmin}, DeletionTimestamp: &deleted},
	}
}
