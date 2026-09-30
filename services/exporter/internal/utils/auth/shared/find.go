package shared

import (
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

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

func FindResourcesByUserID(
	resourceMetadata metadata.Metadata,
	listFormatErr dataerrors.Error,
	userID string,
) ([]unstructured.Unstructured, error) {
	list, err := sharedutils.ListResources(resourceMetadata, listFormatErr)
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
		if status := sharedutils.StatusForError(err, http.StatusNotFound); status != http.StatusNotFound {
			sharedutils.LogByStatusAndSend(w, status, response.OperationError, string(constants.ErrResourceLookupFailed), nil, err)
			return nil, false
		}
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
