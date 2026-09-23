package protection

import (
	"net/http"
	"strconv"

	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/violations"
	"github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
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
		string(messages.SuccessGetRes),
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
