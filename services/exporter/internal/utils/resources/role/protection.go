package role

import (
	"errors"
	"maps"
	"net/http"
	"slices"

	dataerrors "github.com/telark/telark/internal/data/errors"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func ValidateProtectionFlags(existingRole *roledata.AccessRole, body map[string]any, w http.ResponseWriter) bool {
	if existingRole.Protection == nil {
		return true
	}

	denial := lockedChange(existingRole, body)
	if denial == constants.EmptyString {
		return true
	}
	responseutils.LogAndSendResponse(w, http.StatusForbidden, response.OperationError, string(denial), nil, errors.New(string(denial)))
	return false
}

// A lock the same patch lifts no longer holds: lifting it is itself a
// protection change, which authz.GuardRoleReservedFields governs.
func lockedChange(existing *roledata.AccessRole, body map[string]any) dataerrors.Error {
	protection := existing.Protection
	held := func(locked bool, flag string) bool { return locked && !liftedBy(body, flag) }
	_, scopesPatched := body[constants.FieldScopesAndPermissions]
	switch {
	case held(protection.PreventModification, constants.FieldPreventModification) &&
		slices.ContainsFunc(slices.Collect(maps.Keys(body)), func(field string) bool { return field != constants.FieldProtection }):
		return constants.ErrRoleModificationPrevented
	case held(protection.PreventScopeChanges, constants.FieldPreventScopeChanges) && scopesPatched:
		return constants.ErrRoleScopeChangesPrevented
	case held(protection.LockName, constants.FieldLockName) && changes(body, constants.FieldName, existing.Name):
		return constants.ErrRoleNameLocked
	case held(protection.LockCategory, constants.FieldLockCategory) && changes(body, constants.FieldCategoryRef, existing.CategoryRef):
		return constants.ErrRoleCategoryLocked
	default:
		return constants.EmptyString
	}
}

// A merge patch clears a flag with false or null, and every flag with a null protection.
func liftedBy(body map[string]any, flag string) bool {
	raw, present := body[constants.FieldProtection]
	if !present {
		return false
	}
	patch, isObject := raw.(map[string]any)
	if !isObject {
		return raw == nil
	}
	value, set := patch[flag]
	on, isBool := value.(bool)
	return set && (value == nil || (isBool && !on))
}

func changes(body map[string]any, field, current string) bool {
	value, present := body[field]
	return present && value != current
}
