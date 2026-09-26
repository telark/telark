package authz

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	categorydata "github.com/telark/data/classification/category"
	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/data/resources/telarkconfig"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	notifdispatch "github.com/telark/exporter/internal/utils/notifications"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	xauthz "github.com/telark/x-ware/authz"
)

// Editing what a user may do is a different operation from editing their
// profile, though both arrive as one PATCH. Scope and level per field come
// from the product action table; the list fields carry their add and remove
// rules separately, chosen by what the patch actually changes.
var privilegedUserFields = map[string]xauthz.Requirement{
	constants.FieldRoleRefs:  xauthz.Own(roledata.ScopeUsers),
	constants.FieldGroupRefs: xauthz.Own(roledata.ScopeGroups),
	constants.FieldStatus: {
		Scope:    roledata.ScopeUsers,
		MinLevel: roledata.PermissionLevelAdmin,
		Rule:     xauthz.RuleKey(roledata.ScopeUsers, roledata.ActionSuspendUser),
	},
}

// The add and remove rules of the two membership lists, keyed by field.
var (
	addRules = map[string]xauthz.Requirement{
		constants.FieldRoleRefs:  xauthz.Denyable(xauthz.Own(roledata.ScopeUsers), roledata.ActionAttachRoleToUser),
		constants.FieldGroupRefs: xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionAddUserToGroup),
	}
	removeRules = map[string]xauthz.Requirement{
		constants.FieldRoleRefs:  xauthz.Denyable(xauthz.Own(roledata.ScopeUsers), roledata.ActionRemoveRoleFromUser),
		constants.FieldGroupRefs: xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionRemoveUserFromGroup),
	}
)

// An addition is checked against the add rule and a removal against the remove
// rule, so a caller denied one is not denied the other.
func listRequirements(existing *userdata.User, body map[string]any) []xauthz.Requirement {
	current := map[string][]*string{
		constants.FieldRoleRefs:  existing.RoleRefs,
		constants.FieldGroupRefs: existing.GroupRefs,
	}
	var required []xauthz.Requirement
	for field := range addRules {
		if _, present := body[field]; !present {
			continue
		}
		added, removed := notifdispatch.DiffPtrStringSlices(current[field], notifdispatch.ExtractNewRoleIDsFromBody(body, field))
		if len(added) > constants.DefaultInitValue {
			required = append(required, addRules[field])
		}
		if len(removed) > constants.DefaultInitValue {
			required = append(required, removeRules[field])
		}
	}
	return required
}

func callerIdentity(w http.ResponseWriter, r *http.Request) (xauthz.Identity, bool) {
	identity, ok := xauthz.FromContext(r.Context())
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
	}
	return identity, ok
}

func denyForbidden(w http.ResponseWriter, message string) {
	responseutils.LogAndSendResponse(
		w,
		http.StatusForbidden,
		response.OperationForbidden,
		message,
		nil,
		errors.New(message),
	)
}

// GuardUserPatch separates a profile edit from a privilege edit, and refuses
// anyone editing their own privileges: that is what stops self-promotion.
func GuardUserPatch(w http.ResponseWriter, r *http.Request, existing *userdata.User, body map[string]any) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	if !guardIdentityFields(w, identity, existing, body) {
		return false
	}

	required := requirementsIn(privilegedUserFields, body)
	if len(required) == constants.DefaultInitValue {
		// No privileged field is touched, so this is a profile edit: only the
		// account owner may make it.
		if identity.UserID != existing.ID {
			denyForbidden(w, constants.ErrAuthzNotProfileOwner)
			return false
		}
		return true
	}

	for _, requirement := range append(required, listRequirements(existing, body)...) {
		if !xauthz.Allows(identity, requirement) {
			denyForbidden(w, constants.ErrAuthzPrivilegedFieldDenied)
			return false
		}
	}

	if identity.UserID == existing.ID {
		denyForbidden(w, constants.ErrAuthzSelfPrivilegeChange)
		return false
	}

	return addedMembershipsWithinCaller(w, identity, existing, body)
}

func addedMembershipsWithinCaller(w http.ResponseWriter, identity xauthz.Identity, existing *userdata.User, body map[string]any) bool {
	newRoles := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldRoleRefs)
	addedRoles, _ := notifdispatch.DiffPtrStringSlices(existing.RoleRefs, newRoles)
	if !assignedRolesWithinCaller(w, identity, addedRoles) {
		return false
	}
	if _, present := body[constants.FieldGroupRefs]; !present {
		return true
	}
	newGroups := notifdispatch.ExtractNewRoleIDsFromBody(body, constants.FieldGroupRefs)
	addedGroups, _ := notifdispatch.DiffPtrStringSlices(existing.GroupRefs, newGroups)
	return groupsWithinCaller(w, identity, addedGroups)
}

// A new user may carry roles, groups or a status only from a caller who could
// attach them afterwards; empty lists are what the UI sends for a plain user.
func GuardUserCreate(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	if _, present := body[constants.FieldBootstrap]; present {
		denyForbidden(w, constants.ErrAuthzBootstrapFieldReserved)
		return false
	}
	if !guardIdentitiesField(w, nil, body) {
		return false
	}

	privileged := make(map[string]any, len(body))
	for field, value := range body {
		if !isEmptyValue(value) {
			privileged[field] = value
		}
	}
	required := append(requirementsIn(privilegedUserFields, privileged), listRequirements(&userdata.User{}, privileged)...)
	for _, requirement := range required {
		if !xauthz.Allows(identity, requirement) {
			denyForbidden(w, constants.ErrAuthzPrivilegedFieldDenied)
			return false
		}
	}

	return assignedRolesWithinCaller(w, identity, stringsOf(body[constants.FieldRoleRefs])) &&
		groupsWithinCaller(w, identity, stringsOf(body[constants.FieldGroupRefs]))
}

func isEmptyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == constants.EmptyString
	case []any:
		return len(v) == constants.DefaultInitValue
	default:
		return false
	}
}

// Attaching or removing a role rewrites the grants of every member, so it is
// gated like the UI does: groups Owner plus the matching deny rule.
func GuardGroupRolesPatch(w http.ResponseWriter, r *http.Request, existingRoles []string, body map[string]any) bool {
	raw, present := body[constants.FieldRoleRefs]
	if !present {
		return true
	}

	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	added, removed := notifdispatch.DiffStringSlices(existingRoles, stringsOf(raw))
	if len(added) > constants.DefaultInitValue &&
		!xauthz.Allows(identity, xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionAttachRoleToGroup)) {
		denyForbidden(w, constants.ErrAuthzGroupRolesDenied)
		return false
	}
	if len(removed) > constants.DefaultInitValue &&
		!xauthz.Allows(identity, xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionRemoveRoleFromGroup)) {
		denyForbidden(w, constants.ErrAuthzGroupRolesDenied)
		return false
	}

	return assignedRolesWithinCaller(w, identity, added)
}

// Members are gated like the user side of the same membership: groups Owner
// plus the add or remove rule, never on oneself, and a new member receives the
// group's roles, so those are capped like assigning them directly.
func GuardGroupMembersPatch(w http.ResponseWriter, r *http.Request, existing *groupdata.Group, body map[string]any) bool {
	raw, present := body[constants.FieldUserRefs]
	if !present {
		return true
	}

	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	added, removed := notifdispatch.DiffStringSlices(existing.UserRefs, stringsOf(raw))
	if len(added) > constants.DefaultInitValue && !xauthz.Allows(identity, addRules[constants.FieldGroupRefs]) {
		denyForbidden(w, constants.ErrAuthzGroupMembersDenied)
		return false
	}
	if len(removed) > constants.DefaultInitValue && !xauthz.Allows(identity, removeRules[constants.FieldGroupRefs]) {
		denyForbidden(w, constants.ErrAuthzGroupMembersDenied)
		return false
	}
	if slices.Contains(added, identity.UserID) || slices.Contains(removed, identity.UserID) {
		denyForbidden(w, constants.ErrAuthzSelfPrivilegeChange)
		return false
	}
	if len(added) == constants.DefaultInitValue {
		return true
	}

	roles := existing.RoleRefs
	if rawRoles, present := body[constants.FieldRoleRefs]; present {
		roles = stringsOf(rawRoles)
	}
	return assignedRolesWithinCaller(w, identity, roles)
}

// Joining a group hands out every role it carries.
func groupsWithinCaller(w http.ResponseWriter, identity xauthz.Identity, groupIDs []string) bool {
	for _, groupID := range groupIDs {
		group, err := source.Group(groupID)
		if errors.Is(err, xauthz.ErrNotFound) {
			continue
		}
		if err != nil {
			responseutils.LogAndSendResponse(
				w, http.StatusServiceUnavailable, response.OperationUnavailable, string(constants.ErrResourceLookupFailed), nil, err,
			)
			return false
		}
		if !assignedRolesWithinCaller(w, identity, group.RoleRefs) {
			return false
		}
	}
	return true
}

// A reference must resolve to a live record; one held only by the cleanup
// finalizer reads as gone. Unreadable records fail closed.
func GuardReferencedIDs(w http.ResponseWriter, kind string, ids []string) bool {
	var missing []string
	for _, id := range ids {
		deletion, err := lookupDeletion(kind, id)
		if err != nil && !errors.Is(err, xauthz.ErrNotFound) {
			responseutils.LogAndSendResponse(
				w, http.StatusServiceUnavailable, response.OperationUnavailable, string(constants.ErrResourceLookupFailed), nil, err,
			)
			return false
		}
		if err != nil || deletion != nil {
			missing = append(missing, id)
		}
	}
	if len(missing) == constants.DefaultInitValue {
		return true
	}
	message := fmt.Sprintf(constants.ErrAuthzUnknownReferences, kind, strings.Join(missing, constants.ListSeparator))
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, message, nil, errors.New(message))
	return false
}

func lookupDeletion(kind, id string) (*string, error) {
	switch kind {
	case constants.ResourceUser:
		user, err := source.User(id)
		if err != nil {
			return nil, err
		}
		return user.DeletionTimestamp, nil
	case constants.ResourceGroup:
		group, err := source.Group(id)
		if err != nil {
			return nil, err
		}
		return group.DeletionTimestamp, nil
	case constants.ResourceRole:
		role, err := source.Role(id)
		if err != nil {
			return nil, err
		}
		return role.DeletionTimestamp, nil
	default:
		return nil, xauthz.ErrNotFound
	}
}

// The cleanup cascade patches terminating records with the service token;
// everyone else sees them as already gone.
func GuardNotTerminating(w http.ResponseWriter, r *http.Request, deletionTimestamp *string) bool {
	if deletionTimestamp == nil {
		return true
	}

	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	responseutils.LogAndSendResponse(w, http.StatusGone, response.OperationError, constants.ErrAuthzResourceBeingDeleted, nil, nil)
	return false
}

// Assigning a role hands out its levels, so it is capped like authoring one.
// Its deny rules never raise privilege and its status is ignored: an inactive
// role above the caller could be switched on later by someone else.
func assignedRolesWithinCaller(w http.ResponseWriter, identity xauthz.Identity, roleIDs []string) bool {
	for _, roleID := range roleIDs {
		role, err := source.Role(roleID)
		if errors.Is(err, xauthz.ErrNotFound) {
			continue
		}
		if err != nil {
			responseutils.LogAndSendResponse(
				w, http.StatusServiceUnavailable, response.OperationUnavailable, string(constants.ErrResourceLookupFailed), nil, err,
			)
			return false
		}
		if entry, exceeds := levelAboveCaller(identity.Grants, role.ScopesAndPermissions); exceeds {
			denyForbidden(w, fmt.Sprintf(constants.ErrAuthzAssignedRoleExceedsCaller, role.Name, entry.Level, entry.Scope))
			return false
		}
	}
	return true
}

func stringsOf(raw any) []string {
	list, isList := raw.([]any)
	if !isList {
		return nil
	}
	out := make([]string, constants.DefaultInitValue, len(list))
	for _, item := range list {
		if s, isString := item.(string); isString {
			out = append(out, s)
		}
	}
	return out
}

// A role may grant at most what its author holds on each scope; an ALL grant
// counts for every scope, so an Admin on ALL can author anything.
func GuardRoleLevels(w http.ResponseWriter, r *http.Request, scopes []roledata.ScopeAndPermissions) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	if _, exceeds := levelAboveCaller(identity.Grants, scopes); exceeds {
		denyForbidden(w, constants.ErrAuthzRoleLevelExceedsCaller)
		return false
	}

	return true
}

// Status and validity can switch a role's levels back on, so they are capped
// like a scope edit, against the role as it will be stored.
func GuardPatchedRoleLevels(w http.ResponseWriter, r *http.Request, merged *roledata.AccessRole, body map[string]any) bool {
	if !slices.ContainsFunc(constants.RoleLevelFields, func(field string) bool {
		_, patched := body[field]
		return patched
	}) {
		return true
	}
	return GuardRoleLevels(w, r, merged.ScopesAndPermissions)
}

// A session could otherwise mint a role nobody may edit or delete.
func GuardRoleReservedFields(w http.ResponseWriter, r *http.Request, existing *roledata.AccessRole, body map[string]any) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}
	if identity.Internal {
		return true
	}
	if roleType, present := body[constants.FieldType]; present && roleType == string(roledata.RoleTypeBuiltIn) {
		denyForbidden(w, constants.ErrAuthzRoleReservedField)
		return false
	}
	raw, present := body[constants.FieldProtection]
	if !present {
		return true
	}
	current := roledata.Protection{}
	if existing != nil && existing.Protection != nil {
		current = *existing.Protection
	}
	patched, err := sharedutils.ExtractStructFromBody[roledata.AccessRole](map[string]any{constants.FieldProtection: raw})
	if err == nil && (patched.Protection == nil || *patched.Protection == current) {
		return true
	}
	denyForbidden(w, constants.ErrAuthzRoleReservedField)
	return false
}

func levelAboveCaller(grants xauthz.Grants, scopes []roledata.ScopeAndPermissions) (roledata.ScopeAndPermissions, bool) {
	for _, entry := range scopes {
		if !effectiveLevel(grants, entry.Scope).Covers(entry.Level) {
			return entry, true
		}
	}
	return roledata.ScopeAndPermissions{}, false
}

func effectiveLevel(grants xauthz.Grants, scope string) roledata.PermissionLevel {
	level := grants.Levels[scope]
	if all := grants.Levels[roledata.ScopeAll]; all.Rank() > level.Rank() {
		return all
	}
	return level
}

// The UI edits only displayName and description; every other spec field is
// derived or audit data that only discovery and the notifier may write. The
// body is the merge patch itself, so keys beside "spec" (metadata) are checked too.
func GuardApplicationPatch(w http.ResponseWriter, r *http.Request, patch, target map[string]any) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	for field := range target {
		if !slices.Contains(constants.ApplicationUserFields, field) {
			denyForbidden(w, constants.ErrAuthzApplicationFieldDenied)
			return false
		}
	}
	for field := range patch {
		if field != constants.SpecField && !slices.Contains(constants.ApplicationUserFields, field) {
			denyForbidden(w, constants.ErrAuthzApplicationFieldDenied)
			return false
		}
	}

	return true
}

func requirementsIn(fields map[string]xauthz.Requirement, body map[string]any) []xauthz.Requirement {
	required := make([]xauthz.Requirement, constants.DefaultInitValue, len(fields))
	for field, requirement := range fields {
		if _, present := body[field]; present {
			required = append(required, requirement)
		}
	}
	return required
}

// Holding the users scope means administering users, not reading their tokens.
func GuardSelfUser(w http.ResponseWriter, r *http.Request, targetUserID string) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal || identity.UserID == targetUserID {
		return true
	}

	denyForbidden(w, constants.ErrAuthzNotSessionOwner)
	return false
}

// An unknown token is refused like someone elses, so this cannot probe for
// which tokens exist.
func GuardSelfSessionToken(w http.ResponseWriter, r *http.Request, token string) bool {
	return guardSessionOwner(w, r, token, func(w http.ResponseWriter) {
		denyForbidden(w, constants.ErrAuthzNotSessionOwner)
	})
}

// Another account's session answers 404 exactly like a missing one, so a name
// cannot be used to probe which sessions exist.
func GuardOwnSessionName(w http.ResponseWriter, r *http.Request, name string) bool {
	return guardSessionOwner(w, r, name, func(w http.ResponseWriter) {
		responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound,
			string(constants.ErrSessionNotFound), nil, nil)
	})
}

func guardSessionOwner(w http.ResponseWriter, r *http.Request, ref string, deny func(http.ResponseWriter)) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	// Expiry unchecked: a user must still be able to delete an expired session.
	resource, err := sessionutils.FindSessionByRef(ref)
	if err != nil {
		var upstream *sharedutils.UpstreamError
		if errors.As(err, &upstream) {
			sharedutils.LogByStatusAndSend(w, upstream.Status, response.OperationError, string(constants.ErrResourceLookupFailed), nil, err)
			return false
		}
		deny(w)
		return false
	}

	session, err := sessionutils.UnstructuredToSession(resource)
	if err != nil || session.UserID != identity.UserID {
		deny(w)
		return false
	}

	return true
}

// A category is governed by the authz scope it classifies, and each operation
// on it has its own rule.
var categoryRequirements = map[string]map[string]xauthz.Requirement{
	roledata.ScopeGroups: {
		constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeGroups), roledata.ActionAddGroupCategory),
		constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionEditGroupCategory),
		constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeGroups), roledata.ActionDeleteGroupCategory),
	},
	roledata.ScopeRoles: {
		constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeRoles), roledata.ActionAddRoleCategory),
		constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeRoles), roledata.ActionEditRoleCategory),
		constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeRoles), roledata.ActionDeleteRoleCategory),
	},
	categorydata.ScopePlanEnvironments: planCategoryRequirements,
	categorydata.ScopePlanTags:         planCategoryRequirements,
}

var planCategoryRequirements = map[string]xauthz.Requirement{
	constants.CategoryOpCreate: xauthz.Denyable(xauthz.Write(roledata.ScopeProtectionPlans), roledata.ActionAddProtectionPlanCategory),
	constants.CategoryOpEdit:   xauthz.Denyable(xauthz.Own(roledata.ScopeProtectionPlans), roledata.ActionEditProtectionPlanCategory),
	constants.CategoryOpDelete: xauthz.Denyable(xauthz.Own(roledata.ScopeProtectionPlans), roledata.ActionDeleteProtectionPlanCategory),
}

// An unknown scope or operation is refused outright rather than slipping past the check.
func GuardCategoryScope(w http.ResponseWriter, r *http.Request, categoryScope, operation string) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	requirement, known := categoryRequirements[categoryScope][operation]
	if !known {
		denyForbidden(w, constants.ErrAuthzUnknownCategoryScope)
		return false
	}

	if !xauthz.Allows(identity, requirement) {
		denyForbidden(w, constants.ErrAuthzCategoryScopeDenied)
		return false
	}

	return true
}

// One endpoint, but the role model grants its parts separately. Identity
// settings decide who can authenticate at all, so they take Admin.
var configFields = map[string]xauthz.Requirement{
	telarkconfig.FieldExcludedNamespaces: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditDiscoveryConfig),
	},
	telarkconfig.FieldSnapshots: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditSnapshotStorage),
	},
	telarkconfig.FieldAI: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelOwner,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionControlAIInsights),
	},
	telarkconfig.FieldOIDC: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelAdmin,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditOIDCConfig),
	},
	telarkconfig.FieldUserSettings: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditDiscoveryConfig),
	},
	// Written by discovery at startup; no session ever Allows an Internal
	// requirement, since no level covers its empty MinLevel.
	telarkconfig.FieldCluster: xauthz.Internal,
}

// Trusting an identity provider lets whoever controls it sign in as any user,
// so the OIDC settings also take Admin on every scope, not only on settings.
var oidcTrust = xauthz.Administer(roledata.ScopeAll)

// Fields absent from the table are not privileges and stay open.
func GuardConfigPatch(w http.ResponseWriter, r *http.Request, spec map[string]any) bool {
	required := requirementsIn(configFields, spec)
	if _, present := spec[telarkconfig.FieldOIDC]; present {
		required = append(required, oidcTrust)
	}
	if len(required) == constants.DefaultInitValue {
		return true
	}

	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	for _, requirement := range required {
		if !xauthz.Allows(identity, requirement) {
			denyForbidden(w, constants.ErrAuthzConfigDenied)
			return false
		}
	}

	return true
}

// Lifecycle, approval and material plan keys are written only by discovery.
func GuardPlanLifecycle(w http.ResponseWriter, r *http.Request, body map[string]any) bool {
	if !slices.ContainsFunc(constants.PlanLifecycleFields, func(field string) bool {
		_, present := body[field]
		return present
	}) {
		return true
	}

	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal {
		return true
	}

	denyForbidden(w, constants.ErrAuthzPlanLifecycleDenied)
	return false
}

func GuardSnapshotManifestView(w http.ResponseWriter, r *http.Request) bool {
	identity, ok := callerIdentity(w, r)
	if !ok {
		return false
	}

	if identity.Internal || xauthz.Allows(identity, snapshotManifestView) {
		return true
	}

	denyForbidden(w, constants.ErrAuthzSnapshotManifestDenied)
	return false
}

// Enforces protection.preventDeletion, which builtin roles set but nothing read.
func GuardRoleDeletion(w http.ResponseWriter, existingRole *roledata.AccessRole) bool {
	if existingRole.Protection == nil || !existingRole.Protection.PreventDeletion {
		return true
	}

	denyForbidden(w, constants.ErrRoleDeletionPrevented)
	return false
}
