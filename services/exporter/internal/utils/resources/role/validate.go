package role

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	roleconstants "github.com/telark/telark/services/exporter/internal/utils/compute/role/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func ValidateAndPrepareRole(role *roledata.AccessRole, w http.ResponseWriter) error {
	if err := validateRoleFields(role, w); err != nil {
		return err
	}

	return setRoleID(role, w)
}

func validateRoleFields(role *roledata.AccessRole, w http.ResponseWriter) error {
	if err := validateRoleName(role, w); err != nil {
		return err
	}

	if err := validateRoleBasicFields(role, w); err != nil {
		return err
	}

	if err := validateDenyRulesOrRespond(w, role.ScopesAndPermissions); err != nil {
		return err
	}

	return validateRoleComplexFields(role, w)
}

// Only written rules are checked: a stored role with a malformed rule keeps loading.
func validateDenyRulesOrRespond(w http.ResponseWriter, scopes []roledata.ScopeAndPermissions) error {
	for _, scope := range scopes {
		if scope.Rules == nil {
			continue
		}
		for _, rule := range *scope.Rules {
			if !isKnownDenyRule(scope.Scope, rule) {
				err := fmt.Errorf(string(constants.ErrRoleDenyRuleUnknown), rule, scope.Scope)
				responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, err.Error(), nil)
				return err
			}
		}
	}
	return nil
}

// Authz denies a request when its RuleKey is listed under the request's scope or under ALL.
func isKnownDenyRule(scope, rule string) bool {
	for ruleScope, actions := range roledata.DenyableActions {
		if scope != ruleScope && scope != roledata.ScopeAll {
			continue
		}
		if slices.ContainsFunc(actions, func(action string) bool { return xauthz.RuleKey(ruleScope, action) == rule }) {
			return true
		}
	}
	return false
}

func validateRoleName(role *roledata.AccessRole, w http.ResponseWriter) error {
	if err := sharedutils.ValidateRequiredField(role.Name, string(constants.ErrRoleNameCannotBeEmpty)); err != nil {
		responseutils.SendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
		)
		return err
	}
	return nil
}

func validateRoleBasicFields(role *roledata.AccessRole, w http.ResponseWriter) error {
	if role.Description == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleDescriptionRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleDescriptionRequired))
	}

	if role.CategoryRef == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleCategoryRefRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleCategoryRefRequired))
	}

	if len(role.ScopesAndPermissions) == constants.DefaultInitValue {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleScopesAndPermissionsRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleScopesAndPermissionsRequired))
	}

	return nil
}

func validateRoleComplexFields(role *roledata.AccessRole, w http.ResponseWriter) error {
	if role.Validity == nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleValidityRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleValidityRequired))
	}

	if role.Protection == nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleProtectionRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleProtectionRequired))
	}

	return nil
}

func ValidatePriorityCapOrRespond(w http.ResponseWriter, priority int) error {
	if priority >= roleconstants.BuiltInRolePriorityBoost {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRolePriorityExceedsLimit),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRolePriorityExceedsLimit))
	}
	return nil
}

func setRoleID(role *roledata.AccessRole, w http.ResponseWriter) error {
	roleID, err := resourcesshared.GenerateUniqueResourceID(
		metadata.AccessRoleMetadata,
		constants.RoleIDConfig,
	)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return err
	}
	role.ID = roleID
	return nil
}
