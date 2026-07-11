package shared

import (
	"errors"
	"fmt"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListResources(resourceMetadata metadata.Metadata, listFormatErr dataerrors.Error) (*unstructured.UnstructuredList, error) {
	result := api.ListCustomResources(resourceMetadata)
	if result.Error != nil {
		return nil, result.Error
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		return nil, errors.New(string(listFormatErr))
	}

	return list, nil
}

func FilterResourcesByUserID(list *unstructured.UnstructuredList, userID string) []unstructured.Unstructured {
	var resources []unstructured.Unstructured
	for _, item := range list.Items {
		if spec, exists := item.Object[constants.SpecField].(map[string]any); exists {
			if itemUserID, ok := spec[constants.UserIDParam].(string); ok && itemUserID == userID {
				resources = append(resources, item)
			}
		}
	}
	return resources
}

func FindResourceByUserID(
	resourceMetadata metadata.Metadata,
	listFormatErr dataerrors.Error,
	userID string,
) (*unstructured.Unstructured, error) {
	list, err := ListResources(resourceMetadata, listFormatErr)
	if err != nil {
		return nil, err
	}

	resources := FilterResourcesByUserID(list, userID)
	if len(resources) == constants.DefaultInitValue {
		return nil, errors.New(string(dataerrors.ErrGetRes))
	}

	return &resources[constants.DefaultInitValue], nil
}

func FindResourcesByUserID(
	resourceMetadata metadata.Metadata,
	listFormatErr dataerrors.Error,
	userID string,
) ([]unstructured.Unstructured, error) {
	list, err := ListResources(resourceMetadata, listFormatErr)
	if err != nil {
		return nil, err
	}

	return FilterResourcesByUserID(list, userID), nil
}

func FindResourceOrRespond(
	w http.ResponseWriter,
	findFunc func() (*unstructured.Unstructured, error),
	notFoundErr dataerrors.Error,
	formatArgs ...any,
) (*unstructured.Unstructured, bool) {
	resource, err := findFunc()
	if err != nil {
		var errorMsg string
		if len(formatArgs) > constants.DefaultInitValue {
			errorMsg = fmt.Sprintf(string(notFoundErr), formatArgs...)
		} else {
			errorMsg = string(notFoundErr)
		}
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			errorMsg,
			nil,
			err,
		)
		return nil, false
	}
	return resource, true
}
