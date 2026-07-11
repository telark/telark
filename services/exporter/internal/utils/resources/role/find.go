package role

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/constants"
	resourcesshared "github.com/telark/exporter/utils/resources/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ErrRoleNotFound = errors.New(string(constants.ErrRoleNotFound))

func FindRoleByID(roleID string) (*unstructured.Unstructured, error) {
	return resourcesshared.FindResourceByID(roleID, metadata.RoleAsResourceMetadata, constants.ErrRoleNotFound)
}

func FindRoleByIDOrRespond(w http.ResponseWriter, roleID string) (*unstructured.Unstructured, bool) {
	return resourcesshared.FindResourceByIDOrRespond(w, roleID, metadata.RoleAsResourceMetadata, constants.ErrRoleNotFound)
}

func findRoleByAssignedTo(field string, id string) (*unstructured.Unstructured, error) {
	result := api.ListCustomResources(metadata.RoleAsResourceMetadata)
	if result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToListRoles), result.Error)
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	for i := range list.Items {
		item := &list.Items[i]
		spec, ok := item.Object[constants.SpecField].(map[string]any)
		if !ok || spec == nil {
			continue
		}

		assignedTo, ok := spec["assignedTo"].(map[string]any)
		if !ok || assignedTo == nil {
			continue
		}

		ids, ok := assignedTo[field].([]any)
		if !ok {
			continue
		}

		for _, itemID := range ids {
			if idStr, ok := itemID.(string); ok && idStr == id {
				return item, nil
			}
		}
	}

	return nil, ErrRoleNotFound
}

func FindRoleByUserID(userID string) (*unstructured.Unstructured, error) {
	return findRoleByAssignedTo(constants.FieldUserIDs, userID)
}

func FindRoleByUserIDOrRespond(w http.ResponseWriter, userID string) (*unstructured.Unstructured, bool) {
	return findRoleByAssignedToOrRespond(w, constants.FieldUserIDs, userID)
}

func ListRolesByUserID(userID string) ([]*unstructured.Unstructured, error) {
	return findAllRolesByAssignedTo(constants.FieldUserIDs, userID)
}

func ListRolesByUserIDOrRespond(w http.ResponseWriter, userID string) ([]*unstructured.Unstructured, bool) {
	return findAllRolesByAssignedToOrRespond(w, constants.FieldUserIDs, userID)
}

func FindRoleByGroupID(groupID string) (*unstructured.Unstructured, error) {
	return findRoleByAssignedTo(constants.FieldGroupIDs, groupID)
}

func FindRoleByGroupIDOrRespond(w http.ResponseWriter, groupID string) (*unstructured.Unstructured, bool) {
	return findRoleByAssignedToOrRespond(w, constants.FieldGroupIDs, groupID)
}

func ListRolesByGroupID(groupID string) ([]*unstructured.Unstructured, error) {
	return findAllRolesByAssignedTo(constants.FieldGroupIDs, groupID)
}

func ListRolesByGroupIDOrRespond(w http.ResponseWriter, groupID string) ([]*unstructured.Unstructured, bool) {
	return findAllRolesByAssignedToOrRespond(w, constants.FieldGroupIDs, groupID)
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

		assignedTo, ok := spec["assignedTo"].(map[string]any)
		if !ok || assignedTo == nil {
			continue
		}

		ids, ok := assignedTo[field].([]any)
		if !ok {
			continue
		}

		for _, itemID := range ids {
			if idStr, ok := itemID.(string); ok && idStr == id {
				matched = append(matched, item)
				break
			}
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
