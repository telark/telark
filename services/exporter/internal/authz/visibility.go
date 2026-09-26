package authz

import (
	"errors"
	"net/http"
	"slices"

	metadata "github.com/telark/data/metadata/resources"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	userutils "github.com/telark/exporter/internal/utils/resources/user"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	xauthz "github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Restricted reports whether the caller is neither a service nor an Admin on
// ALL: such a caller never learns that an administrator account exists.
func Restricted(r *http.Request) bool {
	identity, ok := xauthz.FromContext(r.Context())
	if !ok {
		return true
	}
	return !identity.Internal && !isAdmin(identity.Grants)
}

func isAdmin(grants xauthz.Grants) bool {
	return grants.Levels[roledata.ScopeAll] == roledata.PermissionLevelAdmin
}

// RestrictedKey blanks the cache key for a restricted caller, so their filtered
// view is never stored under, or served from, the entry everyone else shares.
func RestrictedKey(key func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		if Restricted(r) {
			return constants.EmptyString
		}
		return key(r)
	}
}

// HiddenUsers answers whether a user is an administrator or a bootstrap
// account, memoising role and group lookups across one request. An unreadable
// role or group counts as administrative: hiding on doubt leaks nothing.
func HiddenUsers() func(*userdata.UserAsResource) bool {
	roles := map[string]bool{}
	groups := map[string]bool{}
	return func(user *userdata.UserAsResource) bool {
		return user.Bootstrap ||
			slices.ContainsFunc(user.AssignedRolesIDs, func(id *string) bool { return id != nil && adminRole(roles, *id) }) ||
			slices.ContainsFunc(user.AssignedGroupsIDs, func(id *string) bool { return id != nil && adminGroup(groups, roles, *id) })
	}
}

func adminRole(memo map[string]bool, id string) bool {
	if known, ok := memo[id]; ok {
		return known
	}
	role, err := source.Role(id)
	memo[id] = unreadable(err) || err == nil && xauthz.RoleGrantsAccess(role) && slices.ContainsFunc(role.ScopesAndPermissions,
		func(entry roledata.ScopeAndPermissions) bool {
			return entry.Scope == roledata.ScopeAll && entry.Level == roledata.PermissionLevelAdmin
		})
	return memo[id]
}

func adminGroup(memo, roles map[string]bool, id string) bool {
	if known, ok := memo[id]; ok {
		return known
	}
	group, err := source.Group(id)
	memo[id] = unreadable(err) || err == nil && group.DeletionTimestamp == nil &&
		slices.ContainsFunc(group.AssignedRolesIDs, func(roleID string) bool { return adminRole(roles, roleID) })
	return memo[id]
}

func unreadable(err error) bool {
	return err != nil && !errors.Is(err, xauthz.ErrNotFound)
}

func HiddenUserIDs() (map[string]bool, error) {
	result := api.ListCustomResources(metadata.UserAsResourceMetadata)
	if result.Error != nil {
		return nil, result.Error
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}

	hidden := HiddenUsers()
	ids := map[string]bool{}
	for i := range list.Items {
		user, err := decode[userdata.UserAsResource](&list.Items[i], nil)
		if err == nil && hidden(user) {
			ids[user.ID] = true
		}
	}
	return ids, nil
}

// GuardHiddenUser answers 404 to a restricted caller asking about an
// administrator, the same as for an id that does not exist.
func GuardHiddenUser(w http.ResponseWriter, r *http.Request, user *userdata.UserAsResource) bool {
	if !Restricted(r) || !HiddenUsers()(user) {
		return true
	}
	responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrUserNotFound), nil, nil)
	return false
}

// An id that does not resolve is left to the route's own lookup to answer.
func GuardHiddenUserID(w http.ResponseWriter, r *http.Request, userID string) bool {
	if !Restricted(r) {
		return true
	}
	user, err := source.User(userID)
	if err != nil {
		return true
	}
	return GuardHiddenUser(w, r, user)
}

// GuardUserTarget enforces who may act on an administrator: bootstrap accounts
// are the chart's (only they may edit themselves, nothing deletes them), other
// administrators are deleted or suspended only by a bootstrap account, and
// nobody deletes themselves.
func GuardUserTarget(w http.ResponseWriter, r *http.Request, target *userdata.UserAsResource, body map[string]any, deleting bool) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}
	if identity.UserID != target.ID && !GuardHiddenUser(w, r, target) {
		return false
	}
	if deleting && target.Bootstrap {
		denyForbidden(w, constants.ErrAuthzBootstrapManagedByChart)
		return false
	}
	if identity.Internal {
		return true
	}
	if _, present := body[constants.FieldBootstrap]; present {
		denyForbidden(w, constants.ErrAuthzBootstrapFieldReserved)
		return false
	}
	if identity.UserID == target.ID {
		if deleting {
			denyForbidden(w, constants.ErrAuthzSelfDelete)
		}
		return !deleting
	}
	return guardAdminTarget(w, identity, target, body, deleting)
}

func guardAdminTarget(w http.ResponseWriter, identity xauthz.Identity, target *userdata.UserAsResource, body map[string]any, deleting bool) bool {
	if target.Bootstrap {
		denyForbidden(w, constants.ErrAuthzBootstrapManagedByChart)
		return false
	}
	_, statusChange := body[constants.FieldStatus]
	if (deleting || statusChange) && HiddenUsers()(target) && !callerIsBootstrap(identity) {
		denyForbidden(w, constants.ErrAuthzAdminNeedsBootstrap)
		return false
	}
	return true
}

// A session may not claim a bootstrap administrator's mailbox: auth promotes
// whoever logs in with it.
func GuardReservedEmail(w http.ResponseWriter, r *http.Request, email string) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}
	if identity.Internal || !slices.Contains(envmanager.GetBootstrapAdmins(), userutils.NormalizeEmail(email)) {
		return true
	}
	denyForbidden(w, constants.ErrAuthzBootstrapEmailReserved)
	return false
}

func callerIsBootstrap(identity xauthz.Identity) bool {
	caller, err := source.User(identity.UserID)
	return err == nil && caller.Bootstrap
}
