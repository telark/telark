package generics

import (
	"errors"
	"fmt"
	"net/http"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GenericGetCustomResource(w http.ResponseWriter, name string, md metadata.Metadata) {
	result := api.GetCustomResourceByName(name, md)
	if status := sharedutils.StatusForResult(result); status != http.StatusOK {
		errorMsg := sharedutils.GenerateResourceError(globalerrors.ErrGetRes, name, result.Error)
		if status != http.StatusNotFound {
			responseutils.LogAndSendResponse(w, status, response.OperationError, errorMsg, nil, result.Error)
			return
		}
		lg.Debug(fmt.Sprintf(string(constants.InfExternalDeletionResourceNotFound), name))
		sharedutils.LogDebugAndSend(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			errorMsg,
			nil,
			result.Error,
		)
		return
	}

	resource, ok := result.Data.(*unstructured.Unstructured)
	if !ok {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(globalerrors.ErrGetRes),
			nil,
			errors.New(string(constants.ErrInvalidResourceTypeReturned)),
		)
		return
	}

	sendFilteredResourceResponse(w, resource)
}

func sendFilteredResourceResponse(w http.ResponseWriter, resource *unstructured.Unstructured) {
	filteredResource, err := sharedutils.FilterData(resource)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationUnprocessed,
			string(globalerrors.ErrFilterRes),
			nil,
			err,
		)
		return
	}

	msg := fmt.Sprintf(string(messages.SuccessGetRes), resource.GetName(), resource.GetKind())
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		filteredResource,
		nil,
	)
}

func GenericListCustomResources(w http.ResponseWriter, resourceMetadata metadata.Metadata) {
	GenericListCustomResourcesKeeping(w, resourceMetadata, nil)
}

// keep sees each raw item before filtering and may edit it; false drops it.
func GenericListCustomResourcesKeeping(w http.ResponseWriter, resourceMetadata metadata.Metadata, keep func(*unstructured.Unstructured) bool) {
	result := api.ListCustomResources(resourceMetadata)

	if list, ok := result.Data.(*unstructured.UnstructuredList); ok && keep != nil {
		kept := list.Items[:constants.DefaultInitValue]
		for i := range list.Items {
			if keep(&list.Items[i]) {
				kept = append(kept, list.Items[i])
			}
		}
		list.Items = kept
	}

	filteredList, err := sharedutils.FilterData(result.Data)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationUnprocessed,
			string(globalerrors.ErrFilterRes),
			nil,
			err,
		)
		return
	}

	sharedutils.LogByStatusAndSend(
		w,
		result.Status,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		filteredList,
		result.Error,
	)
}
