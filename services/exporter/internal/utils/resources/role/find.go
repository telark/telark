package role

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	resourcesshared "github.com/telark/exporter/internal/utils/resources/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrRoleNotFound = errors.New(string(constants.ErrRoleNotFound))

func FindRoleByIDOrRespond(w http.ResponseWriter, roleID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, roleID, metadata.RoleAsResourceMetadata, constants.ErrRoleNotFound)
}

func FindRoleByUserIDOrRespond(w http.ResponseWriter, userID string) (*unstructured.Unstructured, bool) {
	return findRoleByAssignedToOrRespond(w, constants.FieldUserIDs, userID)
}

func ListRolesByUserIDOrRespond(w http.ResponseWriter, userID string) ([]*unstructured.Unstructured, bool) {
	return findAllRolesByAssignedToOrRespond(w, constants.FieldUserIDs, userID)
}

func FindRoleByGroupIDOrRespond(w http.ResponseWriter, groupID string) (*unstructured.Unstructured, bool) {
	return findRoleByAssignedToOrRespond(w, constants.FieldGroupIDs, groupID)
}

func ListRolesByGroupIDOrRespond(w http.ResponseWriter, groupID string) ([]*unstructured.Unstructured, bool) {
	return findAllRolesByAssignedToOrRespond(w, constants.FieldGroupIDs, groupID)
}

func findRoleByAssignedTo(field string, id string) (*unstructured.Unstructured, error) {
	matched, err := findAllRolesByAssignedTo(field, id)
	if err != nil {
		return nil, err
	}
	if len(matched) == constants.DefaultInitValue {
		return nil, ErrRoleNotFound
	}

	return matched[constants.DefaultInitValue], nil
}

func findRoleByAssignedToOrRespond(w http.ResponseWriter, field, id string) (*unstructured.Unstructured, bool) {
	resource, err := findRoleByAssignedTo(field, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			responseutils.LogAndSendResponse(
				w,
				http.StatusNotFound,
				response.OperationNotFound,
				string(constants.ErrRoleNotFound),
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

func findAllRolesByAssignedTo(field string, id string) ([]*unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.RoleAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListRoles), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	var matched []*unstructured.Unstructured
	for i := range list.Items {
		item := &list.Items[i]
		spec, ok := item.Object[constants.SpecField].(map[string]any)
		if !ok || spec == nil {
			continue
		}

		assignedTo, ok := spec[constants.FieldAssignedTo].(map[string]any)
		if !ok || assignedTo == nil {
			continue
		}

		ids, ok := assignedTo[field].([]any)
		if !ok {
			continue
		}

		if slices.ContainsFunc(ids, func(itemID any) bool {
			idStr, isString := itemID.(string)
			return isString && idStr == id
		}) {
			matched = append(matched, item)
		}
	}

	return matched, nil
}

func findAllRolesByAssignedToOrRespond(w http.ResponseWriter, field, id string) ([]*unstructured.Unstructured, bool) {
	resources, err := findAllRolesByAssignedTo(field, id)
	if err != nil {
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
	return resources, true
}
