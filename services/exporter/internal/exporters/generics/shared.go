package generics

import (
	"fmt"
	"net/http"

	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
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
