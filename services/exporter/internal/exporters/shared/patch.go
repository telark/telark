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
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
)

func PatchResource(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata) {
	resourceName, err := sharedutils.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
		sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
		return
	}

	patchData, err := requestutils.ParseRequestBody(r)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrRestParseRequestBody), err)
		responseutils.LogAndSendResponse(w, http.StatusUnprocessableEntity, response.OperationError, msg, nil, err)
		return
	}

	lock := concurrency.GetLock(resourceName)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericPatchCustomResource(w, resourceMetadata, resourceName, patchData)
}
