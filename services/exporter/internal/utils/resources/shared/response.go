package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func SendFilteredResourcesResponse(w http.ResponseWriter, resources []*unstructured.Unstructured) {
	filtered := make([]any, constants.DefaultInitValue, len(resources))
	for _, r := range resources {
		f, err := sharedutils.FilterData(r)
		if err != nil {
			responseutils.LogAndSendResponse(
				w,
				http.StatusInternalServerError,
				response.OperationError,
				string(errors.ErrFilterRes),
				nil,
				err,
			)
			return
		}
		filtered = append(filtered, f)
	}

	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		filtered,
		nil,
	)
}

func SendFilteredPatchResponse(w http.ResponseWriter, resource *unstructured.Unstructured) {
	sendFilteredResource(w, resource, messages.SuccessUpdateRes)
}

func SendFilteredResourceResponse(w http.ResponseWriter, resource *unstructured.Unstructured) {
	sendFilteredResource(w, resource, messages.SuccessGetRes)
}

func sendFilteredResource(w http.ResponseWriter, resource *unstructured.Unstructured, success messages.Message) {
	filtered, err := sharedutils.FilterData(resource)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(errors.ErrFilterRes),
			nil,
			err,
		)
		return
	}

	msg := fmt.Sprintf(string(success), resource.GetName(), resource.GetKind())
	responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, msg, filtered)
}
