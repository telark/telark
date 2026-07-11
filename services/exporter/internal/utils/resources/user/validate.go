package user

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/resources"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CheckUserExists(userID string) error {
	return resourcesshared.CheckResourceExists(userID, metadata.UserAsResourceMetadata, constants.ErrUserNotFound)
}

func ValidateUserOrRespond(w http.ResponseWriter, userID string) bool {
	return resourcesshared.ValidateResourceOrRespond(w, userID, metadata.UserAsResourceMetadata, constants.ErrUserNotFound)
}

func CheckUsernameExists(username string) error {
	if err := sharedutils.ValidateRequiredField(username, string(constants.ErrUsernameCannotBeEmpty)); err != nil {
		return err
	}

	exists, err := sharedutils.CheckFieldValueExists(
		metadata.UserAsResourceMetadata,
		dataerrors.ErrGetRes,
		constants.FieldUsername,
		username,
	)
	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf(string(constants.ErrUsernameAlreadyExists), username)
	}

	return nil
}

func CheckIdentityExists(provider, issuer, subject string) error {
	_, err := FindUserByIdentity(provider, issuer, subject)
	if err == nil {
		return fmt.Errorf(string(constants.ErrIdentityAlreadyExists), provider, issuer, subject)
	}
	if errors.Is(err, ErrUserNotFound) {
		return nil
	}
	return err
}

func ValidateAndPrepareUser(user *userdata.UserAsResource, w http.ResponseWriter) error {
	for _, identity := range user.Identities {
		if identity == nil {
			continue
		}
		if err := CheckIdentityExists(identity.Provider, identity.Issuer, identity.Subject); err != nil {
			responseutils.LogAndSendResponse(w, http.StatusConflict, response.OperationAlreadyExists,
				err.Error(), nil, err)
			return err
		}
	}

	if err := CheckUsernameExists(user.Username); err != nil {
		statusCode := http.StatusConflict
		if err.Error() == string(constants.ErrUsernameCannotBeEmpty) {
			statusCode = http.StatusBadRequest
		}
		responseutils.LogAndSendResponse(
			w,
			statusCode,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return err
	}

	userID, err := resourcesshared.GenerateUniqueResourceID(
		metadata.UserAsResourceMetadata,
		constants.UserIDConfig,
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
	user.ID = userID
	return nil
}
