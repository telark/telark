package group

import (
	"encoding/json"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	"github.com/telark/exporter/constants"
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
	spec, ok := resource.Object["spec"].(map[string]any)
	if !ok || spec == nil {
		return nil, nil
	}

	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var group groupdata.GroupAsResource
	if err := json.Unmarshal(specBytes, &group); err != nil {
		return nil, err
	}

	return &group, nil
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
