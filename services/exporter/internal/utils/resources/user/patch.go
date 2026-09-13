package user

import (
	"encoding/json"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetExistingUserForPatch(w http.ResponseWriter, userID string) (*userdata.UserAsResource, bool) {
	existingResource, ok := FindUserByIDOrRespond(w, userID)
	if !ok {
		return nil, false
	}

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

func ExtractUserFromUnstructured(resource *unstructured.Unstructured) (*userdata.UserAsResource, error) {
	spec, ok := resource.Object[constants.SpecField].(map[string]any)
	if !ok || spec == nil {
		return nil, nil
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var user userdata.UserAsResource
	if err := json.Unmarshal(specBytes, &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func ExtractAndMergeUserForPatch(existingUser *userdata.UserAsResource, body map[string]any, w http.ResponseWriter) bool {
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

	MergeUserAndPreparePatchBody(existingUser, newUser, body)
	delete(body, constants.FieldCreationDate)
	return true
}

func MergeUserAndPreparePatchBody(existingUser, newUser *userdata.UserAsResource, body map[string]any) *userdata.UserAsResource {
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
		constants.FieldAssignedRolesIDs,
		newUser.AssignedRolesIDs,
		&mergedUser.AssignedRolesIDs,
	)
	resourcesshared.ReplaceIDsIfProvided(
		body,
		constants.FieldAssignedGroupsIDs,
		newUser.AssignedGroupsIDs,
		&mergedUser.AssignedGroupsIDs,
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
