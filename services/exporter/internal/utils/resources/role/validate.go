package role

import (
	"errors"
	"net/http"

	metadata "github.com/telark/data/metadata/resources"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/constants"
	roleconstants "github.com/telark/exporter/utils/computation/role/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CheckRoleExists(roleID string) error {
	return resourcesshared.CheckResourceExists(roleID, metadata.RoleAsResourceMetadata, constants.ErrRoleNotFound)
}

func ValidateRoleOrRespond(w http.ResponseWriter, roleID string) bool {
	return resourcesshared.ValidateResourceOrRespond(w, roleID, metadata.RoleAsResourceMetadata, constants.ErrRoleNotFound)
}

func ValidateAndPrepareRole(role *roledata.RoleAsResource, w http.ResponseWriter) error {
	if err := validateRoleFields(role, w); err != nil {
		return err
	}

	return setRoleID(role, w)
}

func validateRoleFields(role *roledata.RoleAsResource, w http.ResponseWriter) error {
	if err := validateRoleName(role, w); err != nil {
		return err
	}

	if err := validateRoleBasicFields(role, w); err != nil {
		return err
	}

	return validateRoleComplexFields(role, w)
}

func validateRoleName(role *roledata.RoleAsResource, w http.ResponseWriter) error {
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

func validateRoleBasicFields(role *roledata.RoleAsResource, w http.ResponseWriter) error {
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

	if role.CategoryID == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			string(constants.ErrRoleCategoryIDRequired),
			nil,
			nil,
		)
		return errors.New(string(constants.ErrRoleCategoryIDRequired))
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

func validateRoleComplexFields(role *roledata.RoleAsResource, w http.ResponseWriter) error {
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

func setRoleID(role *roledata.RoleAsResource, w http.ResponseWriter) error {
	roleID, err := resourcesshared.GenerateUniqueResourceID(
		metadata.RoleAsResourceMetadata,
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
