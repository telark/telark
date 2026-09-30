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
)

func PatchResource(w http.ResponseWriter, r *http.Request, resourceMetadata metadata.Metadata) {
	resourceName, err := sharedutils.GetPathParam(w, r, constants.NameParam)
	if err != nil {
		msg := fmt.Sprintf(string(errors.ErrRestRequiredParam), constants.NameParam)
		sharedutils.LogAndReturnError(w, http.StatusBadRequest, msg, err)
		return
	}

	patchData, err := sharedutils.GetSpec(w, r)
	if err != nil {
		return
	}

	lock := concurrency.GetLock(resourceName)
	lock.Lock()
	defer lock.Unlock()
	generics.GenericPatchCustomResource(w, resourceMetadata, resourceName, patchData)
}
