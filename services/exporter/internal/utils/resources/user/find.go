package user

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrUserNotFound = errors.New(string(constants.ErrUserNotFound))

func FindUserByUsername(username string) (*unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.UserAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListUsers), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	for _, item := range list.Items {
		if spec, exists := item.Object[constants.SpecField].(map[string]any); exists {
			if value, ok := spec[constants.FieldUsername].(string); ok && value == username {
				return &item, nil
			}
		}
	}

	return nil, ErrUserNotFound
}

func FindUserByEmail(email string) (*unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.UserAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListUsers), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	for _, item := range list.Items {
		if spec, exists := item.Object[constants.SpecField].(map[string]any); exists {
			if value, ok := spec[constants.FieldEmail].(string); ok && value == email {
				return &item, nil
			}
		}
	}

	return nil, ErrUserNotFound
}

func FindUserByEmailOrRespond(w http.ResponseWriter, email string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByEmail(email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responseutils.LogAndSendResponse(
				w,
				http.StatusNotFound,
				response.OperationNotFound,
				string(constants.ErrUserNotFound),
				nil,
				nil,
			)
			return nil, false
		}
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return nil, false
	}
	return resource, true
}

func FindUserByID(userID string) (*unstructured.Unstructured, error) {
	return resourcesshared.FindResourceByID(userID, metadata.UserAsResourceMetadata, constants.ErrUserNotFound)
}

func FindUserByUsernameOrRespond(w http.ResponseWriter, username string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByUsername(username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responseutils.LogAndSendResponse(
				w,
				http.StatusNotFound,
				response.OperationNotFound,
				string(constants.ErrUserNotFound),
				nil,
				nil,
			)
			return nil, false
		}
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return nil, false
	}
	return resource, true
}

func FindUserByIDOrRespond(w http.ResponseWriter, userID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, userID, metadata.UserAsResourceMetadata, constants.ErrUserNotFound)
}

func FindUserByIdentity(provider, issuer, subject string) (*unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.UserAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListUsers), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	for _, item := range list.Items {
		spec, exists := item.Object[constants.SpecField].(map[string]any)
		if !exists {
			continue
		}
		identities, ok := spec[constants.FieldIdentities].([]any)
		if !ok {
			continue
		}
		for _, entry := range identities {
			id, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if id["provider"] == provider && id["issuer"] == issuer && id["subject"] == subject {
				return &item, nil
			}
		}
	}

	return nil, ErrUserNotFound
}

func FindUserByIdentityOrRespond(w http.ResponseWriter, provider, issuer, subject string) (*unstructured.Unstructured, bool) {
	resource, err := FindUserByIdentity(provider, issuer, subject)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			responseutils.LogAndSendResponse(w, http.StatusNotFound, response.OperationNotFound,
				string(constants.ErrUserNotFound), nil, nil)
			return nil, false
		}
		responseutils.LogAndSendResponse(w, http.StatusInternalServerError, response.OperationError,
			err.Error(), nil, err)
		return nil, false
	}
	return resource, true
}
