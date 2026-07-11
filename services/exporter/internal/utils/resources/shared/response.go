package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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

func SendFilteredResourceResponse(w http.ResponseWriter, resource *unstructured.Unstructured) {
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

	msg := fmt.Sprintf(string(messages.SuccessGetRes), resource.GetName(), resource.GetKind())
	responseutils.SendResponse(w, http.StatusOK, response.OperationSuccess, msg, filtered)
}
