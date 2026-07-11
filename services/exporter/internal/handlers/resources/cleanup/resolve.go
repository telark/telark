package cleanup

import (
	"net/http"

	sharedutils "github.com/telark/exporter/utils/shared"
	restconstants "github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func resolveTargetFromRequest(w http.ResponseWriter, r *http.Request) (resourceTarget, bool) {
	resourceType, err := sharedutils.GetPathParam(w, r, restconstants.TypeParam)
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
