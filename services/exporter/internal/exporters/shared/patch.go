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
	requestutils "github.com/telark/rest/utils/request"
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
		sharedutils.LogAndReturnError(w, http.StatusUnprocessableEntity, string(errors.ErrRestParseRequestBody), err)
		return
	}

	lock := concurrency.GetLock(resourceName)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericPatchCustomResource(w, resourceMetadata, resourceName, patchData)
}
