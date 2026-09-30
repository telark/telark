package cleanup

import (
	"net/http"

	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func resolveTargetFromRequest(w http.ResponseWriter, r *http.Request) (resourceTarget, bool) {
	resourceType, err := sharedutils.GetPathParam(w, r, constants.TypeParam)
	if err != nil {
		return resourceTarget{}, false
	}
	target, ok := lookupTarget(resourceType)
	if !ok {
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError,
			messageUnknownResourceType, nil, nil)
		return resourceTarget{}, false
	}
	return target, true
}
