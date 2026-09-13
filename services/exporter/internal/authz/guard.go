package authz

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	globalconfigresource "github.com/telark/data/resources/globalconfig"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	xauthz "github.com/telark/x-ware/authz"
)

// Editing what a user may do is a different operation from editing their
// profile, though both arrive as one PATCH. Scope and level per field come
// from the product action table.
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

// GuardUserPatch separates a profile edit from a privilege edit, and refuses
// anyone editing their own privileges: that is what stops self-promotion.
func GuardUserPatch(w http.ResponseWriter, r *http.Request, targetUserID string, body map[string]any) bool {
	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	required := privilegesIn(body)
	if len(required) == constants.DefaultInitValue {
		// No privileged field is touched, so this is a profile edit: only the
		// account owner may make it.
		if identity.UserID != targetUserID {
			denyForbidden(w, constants.ErrAuthzNotProfileOwner)
			return false
		}
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

// Holding the users scope means administering users, not reading their tokens.
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

// An unknown token is refused like someone elses, so this cannot probe for
// which tokens exist.
func GuardSelfSessionToken(w http.ResponseWriter, r *http.Request, token string) bool {
	identity, ok := identityOf(r)
	if !ok {
		denyForbidden(w, string(dataerrors.ErrAuthzIdentityMissing))
		return false
	}

	if identity.Internal {
		return true
	}

	// Expiry unchecked: a user must still be able to delete an expired session.
	resource, err := sessionutils.FindSessionByRef(token)
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

// A category names the scope it classifies, and its deny rules live in that
// scope, so both are resolved from the category itself.
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

// An unknown scope is refused outright rather than slipping past the check.
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

// One endpoint, but the role model grants its parts separately. Identity
// settings decide who can authenticate at all, so they take Admin.
var globalConfigFields = map[string]xauthz.Requirement{
	globalconfigresource.FieldExcludedNamespaces: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditDiscoveryConfig),
	},
	globalconfigresource.FieldSnapshots: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelContributor,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditSnapshotStorage),
	},
	globalconfigresource.FieldAI: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelOwner,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionControlAIInsights),
	},
	globalconfigresource.FieldOIDC: {
		Scope:    roledata.ScopeSettings,
		MinLevel: roledata.PermissionLevelAdmin,
		Rule:     xauthz.RuleKey(roledata.ScopeSettings, roledata.ActionEditOIDCConfig),
	},
}

// Reading the provider key takes the same right as changing it. The key is no
// longer on the CR, so callers add it only when this allows — never strip it
// afterwards, which would leak on any missed path.
func MayControlAIInsights(r *http.Request) bool {
	identity, ok := identityOf(r)
	if !ok {
		return false
	}
	if identity.Internal {
		return true
	}
	return xauthz.Allows(identity, globalConfigFields[globalconfigresource.FieldAI])
}

// Fields absent from the table are not privileges and stay open.
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

// Enforces protection.preventDeletion, which builtin roles set but nothing read.
func GuardRoleDeletion(w http.ResponseWriter, existingRole *roledata.RoleAsResource) bool {
	if existingRole.Protection == nil || !existingRole.Protection.PreventDeletion {
		return true
	}

	denyForbidden(w, constants.ErrRoleDeletionPrevented)
	return false
}
