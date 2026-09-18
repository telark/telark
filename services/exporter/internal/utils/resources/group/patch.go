package group

import (
	"net/http"

	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetExistingGroupForPatch(w http.ResponseWriter, groupID string) (*groupdata.GroupAsResource, bool) {
	existingResource, ok := FindGroupByIDOrRespond(w, groupID)
	if !ok {
		return nil, false
	}

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

func ExtractGroupFromUnstructured(resource *unstructured.Unstructured) (*groupdata.GroupAsResource, error) {
	return sharedutils.SpecToStruct[groupdata.GroupAsResource](resource)
}

func ExtractAndMergeGroupForPatch(existingGroup *groupdata.GroupAsResource, body map[string]any, w http.ResponseWriter) bool {
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
