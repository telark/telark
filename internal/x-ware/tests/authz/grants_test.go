package authz

import (
	"errors"
	"testing"

	dataconstants "github.com/telark/data/constants"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/x-ware/authz"
)

const (
	grantsUserID  = "u-00001-0000-0002"
	grantsGroupID = "ug-00001-0000-0001"
	grantsRoleID  = "r-00001-0000-0001"
	deletedAt     = "2026-09-26T10:00:00Z"
)

type mapSource struct {
	users  map[string]*userdata.User
	groups map[string]*groupdata.Group
	roles  map[string]*roledata.AccessRole
}

func (s mapSource) User(id string) (*userdata.User, error) {
	return lookup(s.users, id)
}

func (s mapSource) Group(id string) (*groupdata.Group, error) {
	return lookup(s.groups, id)
}

func (s mapSource) Role(id string) (*roledata.AccessRole, error) {
	return lookup(s.roles, id)
}

func lookup[T any](records map[string]*T, id string) (*T, error) {
	record, ok := records[id]
	if !ok {
		return nil, authz.ErrNotFound
	}
	return record, nil
}

func activeUser(deletion *string) *userdata.User {
	groupID := grantsGroupID
	return &userdata.User{
		ID:                grantsUserID,
		GroupRefs:         []*string{&groupID},
		Status:            userdata.UserStatus{Phase: string(userdata.AccountPhaseActive)},
		DeletionTimestamp: deletion,
	}
}

func ownerRole(deletion *string) *roledata.AccessRole {
	return &roledata.AccessRole{
		ID:                   grantsRoleID,
		Status:               roledata.RoleStatusActive,
		ScopesAndPermissions: []roledata.ScopeAndPermissions{{Scope: roledata.ScopeUsers, Level: roledata.PermissionLevelOwner}},
		DeletionTimestamp:    deletion,
	}
}

func sourceWith(user *userdata.User, group *groupdata.Group, role *roledata.AccessRole) mapSource {
	return mapSource{
		users:  map[string]*userdata.User{grantsUserID: user},
		groups: map[string]*groupdata.Group{grantsGroupID: group},
		roles:  map[string]*roledata.AccessRole{grantsRoleID: role},
	}
}

// A user, group or role held only by the cleanup finalizer grants nothing:
// revocation must not wait for the sweeper to remove the record.
func TestCollectGrantsIgnoresTerminatingRecords(t *testing.T) {
	deleted := deletedAt
	group := &groupdata.Group{ID: grantsGroupID, RoleRefs: []string{grantsRoleID}}

	grants, err := authz.CollectGrants(sourceWith(activeUser(nil), group, ownerRole(nil)), nil, grantsUserID)
	if err != nil || grants.Levels[roledata.ScopeUsers] != roledata.PermissionLevelOwner {
		t.Fatalf("live records: grants = %+v, err = %v", grants, err)
	}

	_, err = authz.CollectGrants(sourceWith(activeUser(&deleted), group, ownerRole(nil)), nil, grantsUserID)
	if !errors.Is(err, authz.ErrUserNotActive) {
		t.Fatalf("terminating user: err = %v, want ErrUserNotActive", err)
	}

	terminatingGroup := &groupdata.Group{ID: grantsGroupID, RoleRefs: []string{grantsRoleID}, DeletionTimestamp: &deleted}
	grants, err = authz.CollectGrants(sourceWith(activeUser(nil), terminatingGroup, ownerRole(nil)), nil, grantsUserID)
	if err != nil || len(grants.Levels) != dataconstants.DefaultInitValue {
		t.Fatalf("terminating group: grants = %+v, err = %v, want none", grants, err)
	}

	grants, err = authz.CollectGrants(sourceWith(activeUser(nil), group, ownerRole(&deleted)), nil, grantsUserID)
	if err != nil || len(grants.Levels) != dataconstants.DefaultInitValue {
		t.Fatalf("terminating role: grants = %+v, err = %v, want none", grants, err)
	}
}

func TestDenyRulesMatchRegardlessOfCase(t *testing.T) {
	const action = "deleteuser"
	rules := []string{"Users.DeleteUser.Deny"}
	role := ownerRole(nil)
	role.ScopesAndPermissions[dataconstants.DefaultInitValue].Rules = &rules
	group := &groupdata.Group{ID: grantsGroupID, RoleRefs: []string{grantsRoleID}}

	grants, err := authz.CollectGrants(sourceWith(activeUser(nil), group, role), nil, grantsUserID)
	if err != nil {
		t.Fatalf("collect grants: %v", err)
	}
	req := authz.Denyable(authz.Own(roledata.ScopeUsers), action)
	if authz.Allows(authz.Identity{UserID: grantsUserID, Grants: grants}, req) {
		t.Fatalf("rule %q written with capitals must still deny %q", rules[0], req.Rule)
	}
}
