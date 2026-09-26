package user

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/v1alpha1"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractUserSpecFromRequestBody(body map[string]any) (*userdata.User, error) {
	user, err := sharedutils.ExtractStructFromBodyIgnoringID[userdata.User](body)
	if err != nil {
		return nil, err
	}

	if user.Status.Phase == constants.EmptyString {
		user.Status.Phase = string(userdata.AccountPhaseActive)
	}

	user.RoleRefs = resourcesshared.DedupePtrIDs(user.RoleRefs)
	user.GroupRefs = resourcesshared.DedupePtrIDs(user.GroupRefs)

	return user, nil
}

func GetUsernameFromUser(userID string) (string, error) {
	userResource := api.GetCustomResourceByName(userID, metadata.UserMetadata)
	if userResource.Error != nil || userResource.Status != http.StatusOK {
		return constants.EmptyString, errors.New(string(constants.ErrUserNotFound))
	}

	user, ok := userResource.Data.(*unstructured.Unstructured)
	if !ok {
		return constants.EmptyString, errors.New(string(constants.ErrUserNotFound))
	}

	userStruct, err := sharedutils.UnstructuredToStruct[userdata.User](
		user,
		dataerrors.ErrGetRes,
		dataerrors.ErrGetRes,
		dataerrors.ErrRestUnmarshalResourceToJSON,
	)
	if err != nil {
		return constants.EmptyString, err
	}

	if userStruct.Username == constants.EmptyString {
		return constants.EmptyString, errors.New(string(constants.ErrUsernameNotFound))
	}

	return userStruct.Username, nil
}
