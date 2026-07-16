package authz

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	xauthz "github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Editing what a user may do is a different operation from editing their
// profile, even though both arrive as one PATCH.
//
// Scope and level per field come from the product's own action table (the
// dashboard's scopeRules): attaching a role is users/Owner, group membership
// is governed by the groups scope, and suspending an account is Admin.
var privilegedUserFields = map[string]xauthz.Requirement{
	constants.FieldAssignedRolesIDs: {
		Scope:    roledata.ScopeUsers,
		MinLevel: roledata.PermissionLevelOwner,
		Rule:     xauthz.RuleKey(roledata.ScopeUsers, roledata.ActionAttachRoleToUser),
	},
	constants.FieldAssignedGroupsIDs: {
		Scope:    roledata.ScopeGroups,
		MinLevel: roledata.PermissionLevelOwner,
		Rule:     xauthz.RuleKey(roledata.ScopeGroups, roledata.ActionAddUserToGroup),
	},
	constants.FieldStatus: {
		Scope:    roledata.ScopeUsers,
		MinLevel: roledata.PermissionLevelAdmin,
		Rule:     xauthz.RuleKey(roledata.ScopeUsers, roledata.ActionSuspendUser),
	},
}

func identityOf(r *http.Request) (xauthz.Identity, bool) {
	return xauthz.FromContext(r.Context())
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

// GuardUserPatch separates a profile edit from a privilege edit. The route's
// Contributor rule covers the former; the latter additionally demands Owner on
// the users scope and refuses to let anyone edit their own privileges, which is
// what stops a self-service promotion to Admin.
func GuardUserPatch(w http.ResponseWriter, r *http.Request, targetUserID string, body map[string]any) bool {
	required := privilegesIn(body)
	if len(required) == constants.DefaultInitValue {
		return true
	}

	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	for _, requirement := range required {
		if !xauthz.Allows(identity, requirement) {
			denyForbidden(w, constants.ErrAuthzPrivilegedFieldDenied)
			return false
		}
	}

	if identity.UserID == targetUserID {
		denyForbidden(w, constants.ErrAuthzSelfPrivilegeChange)
		return false
	}

	return true
}

func privilegesIn(body map[string]any) []xauthz.Requirement {
	required := make([]xauthz.Requirement, constants.DefaultInitValue, len(privilegedUserFields))
	for field, requirement := range privilegedUserFields {
		if _, present := body[field]; present {
			required = append(required, requirement)
		}
	}
	return required
}

// GuardSelfUser restricts a route to the caller's own record. Holding the users
// scope means being allowed to administer users, not to read their live session
// tokens.
func GuardSelfUser(w http.ResponseWriter, r *http.Request, targetUserID string) bool {
	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal || identity.UserID == targetUserID {
		return true
	}

	denyForbidden(w, constants.ErrAuthzNotSessionOwner)
	return false
}

// GuardSelfSessionToken restricts a session route to a token the caller owns.
// An unknown token is refused with the same answer as someone else's, so the
// route cannot be used to test whether a token exists.
func GuardSelfSessionToken(w http.ResponseWriter, r *http.Request, token string) bool {
	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	// Expiry is deliberately not checked: a user must still be able to delete
	// their own expired sessions.
	resource, err := sessionutils.FindSessionByToken(token)
	if err != nil {
		denyForbidden(w, constants.ErrAuthzNotSessionOwner)
		return false
	}

	session, err := sessionutils.UnstructuredToSession(resource)
	if err != nil || session.UserID != identity.UserID {
		denyForbidden(w, constants.ErrAuthzNotSessionOwner)
		return false
	}

	return true
}

// A category names the scope it classifies, and the deny rules that govern it
// live in that scope: adding a category to groups is groups.addgroupcategory,
// to roles it is roles.addrolecategory. The route cannot know which applies, so
// the level and rule are resolved from the category's own scope here.
var categoryActions = map[string]map[roledata.PermissionLevel]string{
	roledata.ScopeGroups: {
		roledata.PermissionLevelContributor: roledata.ActionAddGroupCategory,
		roledata.PermissionLevelOwner:       roledata.ActionEditGroupCategory,
	},
	roledata.ScopeRoles: {
		roledata.PermissionLevelContributor: roledata.ActionAddRoleCategory,
		roledata.PermissionLevelOwner:       roledata.ActionEditRoleCategory,
	},
}

// GuardCategoryScope restricts a category write to holders of the scope that
// category classifies. A scope no role can grant is refused outright, so an
// unrecognized category scope cannot be used to slip past the check.
func GuardCategoryScope(w http.ResponseWriter, r *http.Request, categoryScope string, level roledata.PermissionLevel) bool {
	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	actions, known := categoryActions[categoryScope]
	if !known {
		denyForbidden(w, constants.ErrAuthzUnknownCategoryScope)
		return false
	}

	requirement := xauthz.Requirement{
		Scope:    categoryScope,
		MinLevel: level,
		Rule:     xauthz.RuleKey(categoryScope, actions[level]),
	}
	if !xauthz.Allows(identity, requirement) {
		denyForbidden(w, constants.ErrAuthzCategoryScopeDenied)
		return false
	}

	return true
}

// One endpoint patches the whole GlobalConfig, but the role model grants its
// parts separately: discovery config and snapshot storage are Contributor with
// a rule each, AI insights is Owner. The route's own check is the weakest of
// them, so each field is checked against its own requirement here.
//
// Identity provider settings decide who can authenticate at all and predate the
// rule vocabulary, so they take Admin and carry no rule to withhold.
var globalConfigFields = map[string]xauthz.Requirement{
	constants.FieldExcludedNamespaces: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditDiscoveryConfig),
	},
	constants.FieldSnapshots: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditSnapshotStorage),
	},
	constants.FieldAI: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelOwner,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionControlAIInsights),
	},
	constants.FieldOIDC: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelAdmin,
	},
}

// RedactGlobalConfig strips the parts of the config a caller may not see.
//
// The provider API key is part of AI insights, so reading it takes the same
// right as changing it. Without this it would travel to anyone allowed to read
// the config at all, which is every role: a secret handed out at ReadOnly.
func RedactGlobalConfig(r *http.Request, resource *unstructured.Unstructured) {
	if resource == nil || mayControlAIInsights(r) {
		return
	}

	spec, found := resource.Object[constants.SpecField].(map[string]any)
	if !found {
		return
	}

	ai, found := spec[constants.FieldAI].(map[string]any)
	if !found {
		return
	}

	delete(ai, constants.FieldAPIKey)
}

func mayControlAIInsights(r *http.Request) bool {
	identity, ok := identityOf(r)
	if !ok {
		return false
	}
	if identity.Internal {
		return true
	}
	return xauthz.Allows(identity, globalConfigFields[constants.FieldAI])
}

// GuardGlobalConfigPatch checks every part of the config the body touches.
// Fields absent from the table (a user's own display preferences, the cluster
// version discovery reports) are left to the route's own check.
func GuardGlobalConfigPatch(w http.ResponseWriter, r *http.Request, spec map[string]any) bool {
	required := globalConfigRequirementsIn(spec)
	if len(required) == constants.DefaultInitValue {
		return true
	}

	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	for _, requirement := range required {
		if !xauthz.Allows(identity, requirement) {
			denyForbidden(w, constants.ErrAuthzGlobalConfigDenied)
			return false
		}
	}

	return true
}

func globalConfigRequirementsIn(spec map[string]any) []xauthz.Requirement {
	required := make([]xauthz.Requirement, constants.DefaultInitValue, len(globalConfigFields))
	for field, requirement := range globalConfigFields {
		if _, present := spec[field]; present {
			required = append(required, requirement)
		}
	}
	return required
}

// GuardRoleDeletion enforces protection.preventDeletion, which the built-in
// roles set but nothing checked.
func GuardRoleDeletion(w http.ResponseWriter, existingRole *roledata.RoleAsResource) bool {
	if existingRole.Protection == nil || !existingRole.Protection.PreventDeletion {
		return true
	}

	denyForbidden(w, constants.ErrRoleDeletionPrevented)
	return false
}
