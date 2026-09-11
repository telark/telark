package shared

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func CreateResource(w http.ResponseWriter, resourceMetadata metadata.Metadata, name string, spec map[string]any) {
	lock := concurrency.GetLock(name)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericCreateCustomResource(w, resourceMetadata, name, spec)
}

func GetOrListResource(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata, action string) {
	var mutex sync.Mutex

	switch action {
	case constants.OpGet:
		handleGet(w, r, resourceMetadata, &mutex)
	case constants.OpList:
		handleLists(w, resourceMetadata, &mutex)
	default:
		handleInvalidAction(w, action)
	}
}

func handleGet(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata, _ *sync.Mutex) {
	name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
		sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
		return
	}

	generics.GenericGetCustomResource(w, r, name, resourceMetadata)
}

func handleLists(w http.ResponseWriter, resourceMetadata metadata.Metadata, mutex *sync.Mutex) {
	generics.GenericListCustomResources(w, resourceMetadata, mutex)
}

func handleInvalidAction(w http.ResponseWriter, action string) {
	message := fmt.Sprintf("%s %s", string(errors.ErrInvalidAction), action)
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, message, nil, nil)
}

func GetUniqueResourceFromList(w http.ResponseWriter, _ *http.Request, resourceMetadata metadata.Metadata) {
	result := api.ListCustomResources(resourceMetadata)

	if result.Status != http.StatusOK || result.Error != nil {
		sharedutils.LogByStatusAndSend(
			w,
			result.Status,
			response.OperationError,
			string(errors.ErrGetRes),
			nil,
			result.Error,
		)
		return
	}

	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok {
		responseutils.LogAndSendResponse(
			w,
			http.StatusInternalServerError,
			response.OperationError,
			string(errors.ErrConvertRes),
			nil,
			fmt.Errorf("%s", string(constants.ErrInvalidResourceTypeReturned)),
		)
		return
	}

	if len(list.Items) == constants.DefaultInitValue {
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			string(errors.ErrGetRes),
			nil,
			nil,
		)
		return
	}

	sendFilteredResourceFromList(w, &list.Items[0])
}

func sendFilteredResourceFromList(w http.ResponseWriter, resource *unstructured.Unstructured) {
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
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		msg,
		filtered,
		nil,
	)
}
