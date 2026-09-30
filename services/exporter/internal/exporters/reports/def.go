package reports

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	"github.com/telark/telark/services/exporter/internal/utils/artifact"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

var lg = constants.GetLogger(constants.PrefixMain)

func CreatePlanReport(w http.ResponseWriter, body []byte) {
	var req reportseps.CreatePlanReportRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError,
			fmt.Sprintf(string(dataerrors.ErrRestParseRequestBody), err), nil, err)
		return
	}
	meta, _, err := reportsutil.Create(envmanager.GetReportsPath(), req)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrCreateRes, req.Meta.ID)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessRecordResCreated), meta, nil)
}

func ListPlanReports(w http.ResponseWriter, planID string) {
	if !artifact.IsSafeSegment(planID) {
		sendStoreError(w, reportsutil.ErrBadID, dataerrors.ErrListRes, planID)
		return
	}
	metas, err := reportsutil.List(envmanager.GetReportsPath(), planID)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrGetRes, planID)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessListRes), metas, nil)
}

func ListReports(w http.ResponseWriter, query url.Values) {
	filter, err := reportsutil.ParseListFilter(query)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrListRes, constants.ReportsPlansSubdir)
		return
	}
	metas, total, err := reportsutil.ListAll(envmanager.GetReportsPath(), filter)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrListRes, constants.ReportsPlansSubdir)
		return
	}
	w.Header().Set(reportseps.HeaderTotalCount, strconv.Itoa(total))
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessListRes), metas, nil)
}

// The whole body is in memory before any header goes out, so a read failure
// still yields a clean error envelope instead of a truncated document.
func DownloadPlanReport(w http.ResponseWriter, planID, reportID, format string) {
	data, contentType, err := reportsutil.Load(envmanager.GetReportsPath(), planID, reportID, format)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrGetRes, reportID)
		return
	}
	w.Header().Set(constants.HeaderContentType, contentType)
	w.Header().Set(constants.HeaderContentTypeOptions, constants.ContentTypeOptionsNoSniff)
	w.Header().Set(constants.HeaderCSP, constants.CSPSandbox)
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write(data); werr != nil {
		lg.Error(sharedutils.GenerateResourceError(dataerrors.ErrGetRes, reportID, werr))
	}
}

func PutPlanReportLedger(w http.ResponseWriter, planID string, body []byte) {
	if err := reportsutil.PutLedger(envmanager.GetReportsPath(), planID, body); err != nil {
		sendStoreError(w, err, dataerrors.ErrUpdateRes, planID)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessUpdate), nil, nil)
}

func GetPlanReportLedger(w http.ResponseWriter, planID string) {
	raw, err := reportsutil.GetLedger(envmanager.GetReportsPath(), planID)
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrGetRes, planID)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessOperation), json.RawMessage(raw), nil)
}

func sendStoreError(w http.ResponseWriter, err error, failure dataerrors.Error, id string) {
	switch {
	case errors.Is(err, reportsutil.ErrBadID), errors.Is(err, reportsutil.ErrBadFormat), errors.Is(err, reportsutil.ErrInvalidLedger),
		errors.Is(err, reportsutil.ErrBadFilter):
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, err.Error(), nil, err)
	case errors.Is(err, os.ErrNotExist):
		sharedutils.LogByStatusAndSend(w, http.StatusNotFound, response.OperationNotFound, string(constants.ErrReportNotFound), nil, err)
	default:
		sharedutils.LogByStatusAndSend(w, http.StatusInternalServerError, response.OperationError,
			sharedutils.GenerateResourceError(failure, id, err), nil, err)
	}
}
