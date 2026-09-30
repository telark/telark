package protection

import (
	"net/http"
	"strconv"

	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
)

func Violations(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	query := violations.Query{
		Limit:  parseLimit(r),
		Result: r.URL.Query().Get(protection.QueryParamResult),
	}

	resp, err := svc.ListViolations(r.Context(), planID, query)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		planMessage(messages.SuccessGetRes, planID),
		resp,
		nil,
	)
}

func parseLimit(r *http.Request) int {
	raw := r.URL.Query().Get(protection.QueryParamLimit)
	if raw == constants.EmptyString {
		return constants.DefaultInitValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return constants.DefaultInitValue
	}
	return v
}
