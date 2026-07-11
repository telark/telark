package generics

import (
	"fmt"
	"net/http"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func filterAndRespond(w http.ResponseWriter, resource *unstructured.Unstructured, messageType messages.Message) {
	filteredResource, ferr := sharedutils.FilterData(resource)
	if ferr != nil {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, string(globalerrors.ErrFilterRes), nil, nil)
		return
	}

	msg := fmt.Sprintf(string(messageType), resource.GetName(), resource.GetKind())
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, msg, filteredResource, nil)
}
