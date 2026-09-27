package shared

import (
	"fmt"
	"net/http"

	"github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/exporters/generics"
	"github.com/telark/exporter/internal/utils/concurrency"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CreateResource(w http.ResponseWriter, resourceMetadata metadata.Metadata, name string, spec map[string]any) {
	lock := concurrency.GetLock(name)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericCreateCustomResource(w, resourceMetadata, name, spec)
}

func GetOrListResource(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata, action string) {
	switch action {
	case constants.OpGet:
		handleGet(w, r, resourceMetadata)
	case constants.OpList:
		generics.GenericListCustomResources(w, resourceMetadata)
	default:
		handleInvalidAction(w, action)
	}
}

func handleGet(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata) {
	name, err := sharedutils.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
		sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
		return
	}

	generics.GenericGetCustomResource(w, name, resourceMetadata)
}

func handleInvalidAction(w http.ResponseWriter, action string) {
	message := fmt.Sprintf("%s %s", string(errors.ErrInvalidAction), action)
	responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, message, nil, nil)
}
