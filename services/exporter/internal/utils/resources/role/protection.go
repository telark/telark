package role

import (
	"errors"
	"net/http"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func ValidateProtectionFlags(existingRole *roledata.RoleAsResource, body map[string]any, w http.ResponseWriter) bool {
	if existingRole.Protection == nil {
		return true
	}

	protection := existingRole.Protection
	isModification := len(body) > constants.DefaultInitValue
	if isModification && protection.PreventModification {
		responseutils.LogAndSendResponse(
			w,
			http.StatusForbidden,
			response.OperationError,
			string(constants.ErrRoleModificationPrevented),
			nil,
			errors.New(string(constants.ErrRoleModificationPrevented)),
		)
		return false
	}

	_, isScopeChange := body[constants.FieldScopesAndPermissions]
	if isScopeChange && protection.PreventScopeChanges {
		responseutils.LogAndSendResponse(
			w,
			http.StatusForbidden,
			response.OperationError,
			string(constants.ErrRoleScopeChangesPrevented),
			nil,
			errors.New(string(constants.ErrRoleScopeChangesPrevented)),
		)
		return false
	}

	return true
}
