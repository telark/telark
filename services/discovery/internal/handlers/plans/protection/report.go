package protection

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	"github.com/telark/telark/services/discovery/internal/helpers/shared"
)

func GenerateReport(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	planID, err := shared.GetPathParam(w, r, constants.IDPathParam)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), constants.ProtectionPlanLifecycleTimeout)
	defer cancel()

	meta, err := svc.GenerateReport(ctx, userID, planID)
	if err != nil {
		status, retryAfterSec := ReportErrorStatus(err)
		if retryAfterSec > constants.DefaultInitValue {
			w.Header().Set(constants.HeaderRetryAfter, strconv.Itoa(retryAfterSec))
			respondError(w, status, protection.ErrReportBusy, err)
			return
		}
		respondError(w, status, dataerrors.Error(err.Error()), err)
		return
	}
	responseutils.LogAndSendResponse(
		w,
		http.StatusOK,
		response.OperationSuccess,
		string(messages.SuccessRecordResCreated),
		meta,
		nil,
	)
}

// Busy is a 429, never a 5xx: the dashboard's health breaker counts every >= 500 from
// discovery and would blank the UI after a handful of busy answers.
func ReportErrorStatus(err error) (status int, retryAfterSec int) {
	if errors.Is(err, reports.ErrRenderBusy) {
		return http.StatusTooManyRequests, reports.RenderBusyRetryAfterSec
	}
	return StatusForErr(err), constants.DefaultInitValue
}
