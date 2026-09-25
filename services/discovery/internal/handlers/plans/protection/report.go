package protection

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/helpers/shared"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GenerateReport(w http.ResponseWriter, r *http.Request) {
	svc, ok := readyService(w)
	if !ok {
		return
	}
	userID := r.Header.Get(constants.HeaderUserID)
	if userID == constants.EmptyString {
		respondError(w, http.StatusUnauthorized, protection.ErrUserMissing, nil)
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
