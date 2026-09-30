package reports

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	dataerrors "github.com/telark/telark/internal/data/errors"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	reportsexp "github.com/telark/telark/services/exporter/internal/exporters/reports"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, constants.ReportMaxBodyBytes))
	if err == nil {
		return body, true
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		sharedutils.LogByStatusAndSend(w, http.StatusRequestEntityTooLarge, response.OperationUnprocessed,
			string(constants.ErrReportBodyTooLarge), nil, err)
		return nil, false
	}
	sharedutils.LogByStatusAndSend(w, http.StatusUnprocessableEntity, response.OperationUnprocessed,
		fmt.Sprintf(string(dataerrors.ErrRestParseRequestBody), err), nil, err)
	return nil, false
}

func CreatePlanReport() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		reportsexp.CreatePlanReport(w, body)
	}
}

func ListReports() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		reportsexp.ListReports(w, r.URL.Query())
	}
}

func ListPlanReports() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		reportsexp.ListPlanReports(w, id)
	}
}

func DownloadPlanReport() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		reportsexp.DownloadPlanReport(w, id, r.URL.Query().Get(reportseps.QueryReport), r.URL.Query().Get(reportseps.QueryFormat))
	}
}

func PutPlanReportLedger() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		body, ok := readBody(w, r)
		if !ok {
			return
		}
		reportsexp.PutPlanReportLedger(w, id, body)
	}
}

func GetPlanReportLedger() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := sharedutils.GetPathParam(w, r, constants.IDParam)
		if err != nil {
			return
		}
		reportsexp.GetPlanReportLedger(w, id)
	}
}
