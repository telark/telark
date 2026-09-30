package user

import (
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	resourcesshared "github.com/telark/telark/services/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetExistingUserForPatch(w http.ResponseWriter, userID string) (*userdata.User, bool) {
	existingResource, ok := FindUserByIDOrRespond(w, userID)
	if !ok {
		return nil, false
	}

	sharedutils.ProjectDeletionTimestamp(existingResource)
	existingUser, err := ExtractUserFromUnstructured(existingResource)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(dataerrors.ErrRestUnmarshalResourceToJSON),
			nil,
			err,
		)
		return nil, false
	}

	return existingUser, true
}

func ExtractUserFromUnstructured(resource *unstructured.Unstructured) (*userdata.User, error) {
	return sharedutils.SpecToStruct[userdata.User](resource)
}

func ExtractAndMergeUserForPatch(existingUser *userdata.User, body map[string]any, w http.ResponseWriter) bool {
	newUser, err := ExtractUserSpecFromRequestBody(body)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return false
	}

	if _, provided := body[constants.FieldUsername]; provided {
		if !CheckUsernameChangeAllowed(existingUser.Username, newUser.Username, w) {
			return false
		}
	}
	if _, provided := body[constants.FieldEmail]; provided {
		if !CheckEmailChangeAllowed(existingUser.Email, newUser.Email, w) {
			return false
		}
	}

	MergeUserAndPreparePatchBody(existingUser, newUser, body)
	delete(body, constants.FieldCreationDate)
	return true
}

func MergeUserAndPreparePatchBody(existingUser, newUser *userdata.User, body map[string]any) *userdata.User {
	mergedUser := *existingUser

	replaceStringIfProvided(body, constants.FieldUsername, newUser.Username, &mergedUser.Username)
	replaceStringIfProvided(body, constants.FieldFullname, newUser.Fullname, &mergedUser.Fullname)
	replaceStringIfProvided(body, constants.FieldEmail, newUser.Email, &mergedUser.Email)

	if _, provided := body[constants.FieldAvatar]; provided {
		mergedUser.Avatar = newUser.Avatar
		body[constants.FieldAvatar] = newUser.Avatar
	}

	if _, provided := body[constants.FieldSettings]; provided {
		mergedUser.Settings = newUser.Settings
		body[constants.FieldSettings] = newUser.Settings
	}

	if _, provided := body[constants.FieldStatus]; provided {
		mergedUser.Status = newUser.Status
		body[constants.FieldStatus] = newUser.Status
	}

	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldRoleRefs,
		newUser.RoleRefs,
		&mergedUser.RoleRefs,
	)
	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldGroupRefs,
		newUser.GroupRefs,
		&mergedUser.GroupRefs,
	)

	return &mergedUser
}

func replaceStringIfProvided(body map[string]any, key string, newValue string, target *string) {
	if _, provided := body[key]; !provided {
		return
	}

	*target = newValue
	body[key] = newValue
}
