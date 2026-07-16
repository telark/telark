package authz

import (
	"errors"
	"fmt"
	"time"

	dataconstants "github.com/telark/data/constants"
	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
)

// A host that owns these records reads them directly; one that does not fetches them.
type GrantSource interface {
	User(userID string) (*userdata.UserAsResource, error)
	Group(groupID string) (*groupdata.GroupAsResource, error)
	Role(roleID string) (*roledata.RoleAsResource, error)
}

// An unreadable role or group is skipped, not fatal: one broken reference must
// not lock every user out, nor pass silently.
type Warner interface {
	Warn(message string)
}

// Flattens every role a user holds into the strongest level per scope. Here, so
// the rules deciding what a role grants exist once rather than once per service.
func CollectGrants(source GrantSource, log Warner, userID string) (Grants, error) {
	user, err := source.User(userID)
	if err != nil {
		return Grants{}, err
	}

	if userdata.AccountPhase(user.Status.Phase) != userdata.AccountPhaseActive {
		return Grants{}, errors.New(string(dataerrors.ErrAuthzUserNotActive))
	}

	grants := Grants{
		Levels: map[string]roledata.PermissionLevel{},
		Denied: map[string][]string{},
	}

	for _, roleID := range roleIDsFor(source, log, user) {
		applyRole(&grants, source, log, roleID)
	}

	return grants, nil
}

func roleIDsFor(source GrantSource, log Warner, user *userdata.UserAsResource) []string {
	roleIDs := make([]string, dataconstants.DefaultInitValue, len(user.AssignedRolesIDs))

	for _, roleID := range user.AssignedRolesIDs {
		if roleID != nil {
			roleIDs = append(roleIDs, *roleID)
		}
	}

	for _, groupID := range user.AssignedGroupsIDs {
		if groupID == nil {
			continue
		}
		group, err := source.Group(*groupID)
		if err != nil {
			warnf(log, dataerrors.ErrAuthzGroupSkipped, *groupID, err)
			continue
		}
		roleIDs = append(roleIDs, group.AssignedRolesIDs...)
	}

	return roleIDs
}

func applyRole(grants *Grants, source GrantSource, log Warner, roleID string) {
	role, err := source.Role(roleID)
	if err != nil {
		warnf(log, dataerrors.ErrAuthzRoleSkipped, roleID, err)
		return
	}

	if !RoleGrantsAccess(role) {
		return
	}

	for _, scope := range role.ScopesAndPermissions {
		MergeLevel(grants.Levels, scope.Scope, scope.Level)
		if scope.Rules != nil && len(*scope.Rules) > dataconstants.DefaultInitValue {
			grants.Denied[scope.Scope] = append(grants.Denied[scope.Scope], *scope.Rules...)
		}
	}
}

// Only an Active, unexpired role confers anything.
func RoleGrantsAccess(role *roledata.RoleAsResource) bool {
	if role.Status != roledata.RoleStatusActive {
		return false
	}
	return !roleExpired(role)
}

// An unparsable expiry counts as expired, never as permanent.
func roleExpired(role *roledata.RoleAsResource) bool {
	if role.Validity == nil || role.Validity.Type != roledata.ValidityTypeTemporary {
		return false
	}
	if role.Validity.ExpiresAt == nil {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, *role.Validity.ExpiresAt)
	if err != nil {
		return true
	}

	return time.Now().After(expiry)
}

func warnf(log Warner, format dataerrors.Error, id string, cause error) {
	if log == nil {
		return
	}
	log.Warn(fmt.Sprintf(string(format), id, cause))
}
