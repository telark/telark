package group

import (
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	groupdata "github.com/telark/telark/internal/data/resources/group"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetExistingGroupForPatch(w http.ResponseWriter, groupID string) (*groupdata.Group, bool) {
	existingResource, ok := FindGroupByIDOrRespond(w, groupID)
	if !ok {
		return nil, false
	}

	sharedutils.ProjectDeletionTimestamp(existingResource)
	existingGroup, err := ExtractGroupFromUnstructured(existingResource)
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

	return existingGroup, true
}

func ExtractGroupFromUnstructured(resource *unstructured.Unstructured) (*groupdata.Group, error) {
	return sharedutils.SpecToStruct[groupdata.Group](resource)
}

func ExtractAndMergeGroupForPatch(existingGroup *groupdata.Group, body map[string]any, w http.ResponseWriter) bool {
	newGroup, err := ExtractGroupSpecFromRequestBody(body)
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

	MergeGroupAndPreparePatchBody(existingGroup, newGroup, body)
	delete(body, constants.FieldCreationDate)
	return true
}
