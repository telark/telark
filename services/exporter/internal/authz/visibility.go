package authz

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/cache"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	userutils "github.com/telark/telark/services/exporter/internal/utils/resources/user"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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

// Every restricted caller sees the same filtered list, so they share one entry
// and one render. Who is hidden follows users, groups and roles, so each moves the key.
func RestrictedListKey(generations cache.ListGenerationReader, key func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		shared := key(r)
		if shared == constants.EmptyString || !Restricted(r) {
			return shared
		}
		return strings.Join([]string{
			shared, constants.CacheRestrictedSegment,
			generations.ListGeneration(constants.ResourceUser),
			generations.ListGeneration(constants.ResourceGroup),
			generations.ListGeneration(constants.ResourceRole),
		}, constants.CacheKeySeparator)
	}
}

// HiddenUsers answers whether a user is an administrator or a bootstrap
// account, memoising role and group lookups across one request. An unreadable
// role or group counts as administrative: hiding on doubt leaks nothing.
func HiddenUsers() func(*userdata.User) bool {
	return hiddenBy(map[string]bool{}, map[string]bool{})
}

func hiddenBy(roles, groups map[string]bool) func(*userdata.User) bool {
	return func(user *userdata.User) bool {
		return user.Bootstrap ||
			slices.ContainsFunc(user.RoleRefs, func(id *string) bool { return id != nil && adminRole(roles, *id) }) ||
			slices.ContainsFunc(user.GroupRefs, func(id *string) bool { return id != nil && adminGroup(groups, roles, *id) })
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
		slices.ContainsFunc(group.RoleRefs, func(roleID string) bool { return adminRole(roles, roleID) })
	return memo[id]
}

func unreadable(err error) bool {
	return err != nil && !errors.Is(err, xauthz.ErrNotFound)
}

func HiddenUserIDs() (map[string]bool, error) {
	users, err := listUsers()
	if err != nil {
		return nil, err
	}

	hidden := HiddenUsers()
	ids := map[string]bool{}
	for _, user := range users {
		if hidden(user) {
			ids[user.ID] = true
		}
	}
	return ids, nil
}

func listUsers() ([]*userdata.User, error) {
	result := api.ListCustomResources(metadata.UserMetadata)
	if result.Error != nil {
		return nil, result.Error
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(constants.ErrInvalidResourceTypeReturned))
	}

	users := make([]*userdata.User, constants.DefaultInitValue, len(list.Items))
	for i := range list.Items {
		if user, err := decode[userdata.User](&list.Items[i], nil); err == nil {
			users = append(users, user)
		}
	}
	return users, nil
}

// GuardHiddenUser answers 404 to a restricted caller asking about an
// administrator, the same as for an id that does not exist.
func GuardHiddenUser(w http.ResponseWriter, r *http.Request, user *userdata.User) bool {
	if !Restricted(r) || !HiddenUsers()(user) {
		return true
	}
	responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrUserNotFound), nil, nil)
	return false
}

// GuardUserTarget enforces who may act on an administrator: bootstrap accounts
// are the chart's (only they may edit themselves, nothing deletes them), and
// nobody deletes themselves.
func GuardUserTarget(w http.ResponseWriter, r *http.Request, target *userdata.User, body map[string]any, deleting bool) bool {
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
	if target.Bootstrap {
		denyForbidden(w, constants.ErrAuthzBootstrapManagedByChart)
		return false
	}
	return true
}

// The bootstrap account and the BOOTSTRAP_ADMIN mailbox stay together (break-glass finds the account by it):
// a session moves neither another account onto it nor the bootstrap account off it. target is nil on create.
func GuardReservedEmail(w http.ResponseWriter, r *http.Request, target *userdata.User, email string) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}
	normalized := userutils.NormalizeEmail(email)
	if identity.Internal || target != nil && normalized == userutils.NormalizeEmail(target.Email) {
		return true
	}
	bootstrap := envmanager.GetBootstrapAdmin()
	reserved := bootstrap != constants.EmptyString && normalized == bootstrap
	bootstrapTarget := target != nil && target.Bootstrap
	switch {
	case bootstrapTarget && !reserved:
		denyForbidden(w, constants.ErrAuthzBootstrapEmailLocked)
	case reserved && !bootstrapTarget:
		denyForbidden(w, constants.ErrAuthzBootstrapEmailReserved)
	default:
		return true
	}
	return false
}

func GuardUserPatchLastAdmin(w http.ResponseWriter, existing *userdata.User, body map[string]any) bool {
	if !slices.ContainsFunc(constants.AdminFields, func(field string) bool {
		_, patched := body[field]
		return patched
	}) {
		return true
	}
	patched, err := sharedutils.ExtractStructFromBody[userdata.User](body)
	if err != nil {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
		return false
	}
	after := *existing
	if _, present := body[constants.FieldStatus]; present && !userutils.StatusKeepsPhase(body) {
		after.Status = patched.Status
	}
	if _, present := body[constants.FieldRoleRefs]; present {
		after.RoleRefs = patched.RoleRefs
	}
	if _, present := body[constants.FieldGroupRefs]; present {
		after.GroupRefs = patched.GroupRefs
	}
	// Only a change that takes an active admin's rights away needs the user list.
	admin := HiddenUsers()
	if !activeAdmin(admin, existing) || activeAdmin(admin, &after) {
		return true
	}
	return guardLastAdmin(w, replacing(existing.ID, &after), nil)
}

func GuardUserDeleteLastAdmin(w http.ResponseWriter, target *userdata.User) bool {
	if !activeAdmin(HiddenUsers(), target) {
		return true
	}
	return guardLastAdmin(w, replacing(target.ID, nil), nil)
}

func GuardGroupPatchLastAdmin(w http.ResponseWriter, existing *groupdata.Group, body map[string]any, removedMembers []string) bool {
	raw, rolesPatched := body[constants.FieldRoleRefs]
	if !rolesPatched && len(removedMembers) == constants.DefaultInitValue {
		return true
	}
	after := *existing
	if rolesPatched {
		after.RoleRefs = stringsOf(raw)
	}
	return guardGroupLastAdmin(w, &after, removedMembers)
}

// A deleted group grants nothing, as if it had no roles.
func GuardGroupDeleteLastAdmin(w http.ResponseWriter, groupID string) bool {
	return guardGroupLastAdmin(w, &groupdata.Group{ID: groupID}, nil)
}

func guardGroupLastAdmin(w http.ResponseWriter, after *groupdata.Group, removedMembers []string) bool {
	return guardLastAdmin(w, func(user *userdata.User) *userdata.User {
		if !slices.Contains(removedMembers, user.ID) {
			return user
		}
		left := *user
		left.GroupRefs = slices.DeleteFunc(slices.Clone(user.GroupRefs), func(id *string) bool { return id != nil && *id == after.ID })
		return &left
	}, after)
}

func replacing(userID string, after *userdata.User) func(*userdata.User) *userdata.User {
	return func(user *userdata.User) *userdata.User {
		if user.ID == userID {
			return after
		}
		return user
	}
}

// Refuses (409) a change leaving no active administrator; edit returns each user as the change leaves it (nil once deleted).
// ponytail: check-then-act, two concurrent removals of the last two admins can both pass; a global lock would close it.
func guardLastAdmin(w http.ResponseWriter, edit func(*userdata.User) *userdata.User, group *groupdata.Group) bool {
	users, err := listUsers()
	if err != nil {
		responseutils.LogAndSendResponse(
			w, http.StatusServiceUnavailable, response.OperationUnavailable, string(constants.ErrResourceLookupFailed), nil, err,
		)
		return false
	}
	roles, groups := map[string]bool{}, map[string]bool{}
	before := hiddenBy(roles, map[string]bool{})
	if group != nil {
		groups[group.ID] = slices.ContainsFunc(group.RoleRefs, func(roleID string) bool { return adminRole(roles, roleID) })
	}
	after := hiddenBy(roles, groups)
	if slices.ContainsFunc(users, func(user *userdata.User) bool { return activeAdmin(after, edit(user)) }) ||
		!slices.ContainsFunc(users, func(user *userdata.User) bool { return activeAdmin(before, user) }) {
		return true
	}
	responseutils.LogAndSendResponse(w, http.StatusConflict, response.OperationError, constants.ErrAuthzLastAdmin, nil, nil)
	return false
}

func activeAdmin(admin func(*userdata.User) bool, user *userdata.User) bool {
	return user != nil && user.DeletionTimestamp == nil &&
		userdata.AccountPhase(user.Status.Phase) == userdata.AccountPhaseActive && admin(user)
}
