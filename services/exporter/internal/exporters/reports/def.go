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
	"github.com/telark/telark/internal/kcore/resources/core"
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

func ListKeptReports(w http.ResponseWriter) {
	kept, err := reportsutil.ListKept(envmanager.GetReportsPath())
	if err != nil {
		sendStoreError(w, err, dataerrors.ErrListRes, constants.ReportsKeptSubdir)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessListRes), kept, nil)
}

// Same shape as the snapshot infos, so the UI reads both volumes alike. A capacity it can't read
// is zero, and the percentages are then left out.
func ReadReportsStorage(w http.ResponseWriter) {
	root := envmanager.GetReportsPath()
	used, count, err := reportsutil.Usage(root)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnReportsStorageWalk), root, err))
	}
	total := reportsPVCBytes()
	free := uint64(constants.DefaultInitValue)
	if total > used {
		free = total - used
	}
	resp := map[string]any{
		constants.FieldTotalPVCSpace:  storageSpace(total, uint64(constants.DefaultInitValue)),
		constants.FieldConsumedSpace:  storageSpace(used, total),
		constants.FieldAvailableSpace: storageSpace(free, total),
		constants.FieldTotalReports:   count,
		constants.FieldPVCName:        envmanager.GetReportsPVCName(),
		constants.FieldPVCNamespace:   envmanager.GetReportsPVCNamespace(),
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(constants.InfReportsStorageRetrieved), resp, nil)
}

func reportsPVCBytes() uint64 {
	name, namespace := envmanager.GetReportsPVCName(), envmanager.GetReportsPVCNamespace()
	bytes, err := core.GetPersistentVolumeClaimCapacityBytes(namespace, name)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnReportsPVCGetFailed), name, namespace, err))
		return uint64(constants.DefaultInitValue)
	}
	if bytes < int64(constants.DefaultInitValue) {
		return uint64(constants.DefaultInitValue)
	}
	return uint64(bytes)
}

func storageSpace(bytes, total uint64) map[string]any {
	kb := float64(bytes) / constants.SnapshotBytesPerKilobyte
	space := map[string]any{
		constants.FieldBytes:     bytes,
		constants.FieldKilobytes: kb,
		constants.FieldMegabytes: kb / constants.SnapshotKilobytesPerMegabyte,
	}
	if total > uint64(constants.DefaultInitValue) {
		space[constants.FieldPercent] = float64(bytes) * constants.SnapshotPercentScale / float64(total)
	}
	return space
}

func DeletePlanReport(w http.ResponseWriter, planID, reportID string) {
	if err := reportsutil.RemoveReport(envmanager.GetReportsPath(), planID, reportID); err != nil {
		sendStoreError(w, err, dataerrors.ErrDeleteRes, reportID)
		return
	}
	responseutils.LogAndSendResponse(w, http.StatusOK, response.OperationSuccess, string(messages.SuccessOperation), nil, nil)
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
