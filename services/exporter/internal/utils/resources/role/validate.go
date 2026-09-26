package role

import (
	"errors"
	"net/http"

	metadata "github.com/telark/data/metadata/v1alpha1"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	roleconstants "github.com/telark/exporter/internal/utils/compute/role/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

	return validateRoleComplexFields(role, w)
}

func validateRoleName(role *roledata.AccessRole, w http.ResponseWriter) error {
	if err := sharedutils.ValidateRequiredField(role.Name, string(constants.ErrRoleNameCannotBeEmpty)); err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			err,
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
