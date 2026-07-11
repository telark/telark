package generics

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/constants"
	sharedutils "github.com/telark/exporter/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GenericGetCustomResource(w http.ResponseWriter, _ *http.Request, name string, md metadata.Metadata) {
	result := api.GetCustomResourceByName(name, md)
	if result.Status != http.StatusOK || result.Error != nil {
		lg.Error(fmt.Sprintf(string(constants.InfExternalDeletionResourceNotFound), name))
		errorMsg := sharedutils.GenerateResourceError(globalerrors.ErrGetRes, name, result.Error)
		responseutils.LogAndSendResponse(
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

func GenericListCustomResources(w http.ResponseWriter, resourceMetadata metadata.Metadata, mutex *sync.Mutex) {
	mutex.Lock()
	defer mutex.Unlock()

	result := api.ListCustomResources(resourceMetadata)

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

	responseutils.LogAndSendResponse(
		w,
		result.Status,
		response.OperationSuccess,
		string(messages.SuccessListRes),
		filteredList,
		result.Error,
	)
}
