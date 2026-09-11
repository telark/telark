package shared

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
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

// The identifier being looked up is a session token or a credential ID, so it
// is never echoed into the message.
func FindResourceOrRespond(
	w http.ResponseWriter,
	findFunc func() (*unstructured.Unstructured, error),
	notFoundErr dataerrors.Error,
) (*unstructured.Unstructured, bool) {
	resource, err := findFunc()
	if err != nil {
		sharedutils.LogDebugAndSend(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			string(notFoundErr),
			nil,
			err,
		)
		return nil, false
	}
	return resource, true
}
