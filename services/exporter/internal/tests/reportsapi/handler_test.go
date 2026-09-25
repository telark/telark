package reportsapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/telark/exporter/internal/authz"
	"github.com/telark/exporter/internal/constants"
	reportshandler "github.com/telark/exporter/internal/handlers/reports"
	"github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/exporter/internal/routes"
	"github.com/telark/exporter/internal/utils/performance"
	reportsutil "github.com/telark/exporter/internal/utils/reports"
	reportseps "github.com/telark/rest/endpoints/reports"
	"github.com/telark/rest/router"
)

const (
	planID       = "plan-a"
	reportID     = "20260101T000000Z-end"
	actor        = "alice"
	otherActor   = "bob"
	generatedAt  = "2026-01-01T00:00:00Z"
	unknownID    = "20260101T000000Z-manual-99"
	pdfFormat    = "pdf"
	ledgerBody   = `{"planId":"plan-a","violations":[]}`
	overLimit    = 33 << 20
	statusFmt    = "%s code = %d, want %d"
	headerFmt    = "%s header %s = %q, want %q"
	routePath    = "/reports"
	downloadName = "download"
	firstIdx     = 0
	reportsCount = 6
	totalOne     = "1"
	badTrigger   = "pdf"
)

var reportRoutes = []string{
	router.Key(http.MethodGet, reportseps.ListReports),
	router.Key(http.MethodPost, reportseps.CreatePlanReport),
	router.Key(http.MethodGet, reportseps.ListPlanReports),
	router.Key(http.MethodGet, reportseps.DownloadPlanReport),
	router.Key(http.MethodPost, reportseps.PutPlanReportLedger),
	router.Key(http.MethodGet, reportseps.GetPlanReportLedger),
}

type envelope struct {
	Data json.RawMessage `json:"data"`
}

func setRoot(t *testing.T) string {
	t.Helper()
	t.Setenv(constants.ReportsPathEnv, t.TempDir())
	return envs.InitReportsPath()
}

func withID(r *http.Request) *http.Request {
	return mux.SetURLVars(r, map[string]string{constants.IDParam: planID})
}

func files(tag string) map[string]string {
	out := make(map[string]string, len(reportseps.Formats))
	for _, f := range reportseps.Formats {
		out[f] = tag + "-" + f
	}
	return out
}

func createBody(t *testing.T, by string) string {
	t.Helper()
	raw, err := json.Marshal(reportseps.CreatePlanReportRequest{
		Meta:  reportseps.ReportMeta{ID: reportID, PlanID: planID, Trigger: reportseps.TriggerEnd, GeneratedAt: generatedAt, GeneratedBy: by},
		Files: files(by),
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func create(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	reportshandler.CreatePlanReport()(rec, httptest.NewRequest(http.MethodPost, routePath, strings.NewReader(body)))
	return rec
}

func decodeData[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("envelope decode: %v", err)
	}
	var out T
	if err := json.Unmarshal(env.Data, &out); err != nil {
		t.Fatalf("data decode: %v", err)
	}
	return out
}

func download(report, format string) *httptest.ResponseRecorder {
	q := url.Values{}
	q.Set(reportseps.QueryReport, report)
	q.Set(reportseps.QueryFormat, format)
	rec := httptest.NewRecorder()
	reportshandler.DownloadPlanReport()(rec, withID(httptest.NewRequest(http.MethodGet, routePath+"?"+q.Encode(), nil)))
	return rec
}

func TestCreateThenListThenDownloadEachFormat(t *testing.T) {
	setRoot(t)
	if rec := create(t, createBody(t, actor)); rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "create", rec.Code, http.StatusOK)
	}

	list := httptest.NewRecorder()
	reportshandler.ListPlanReports()(list, withID(httptest.NewRequest(http.MethodGet, routePath, nil)))
	if list.Code != http.StatusOK {
		t.Fatalf(statusFmt, "list", list.Code, http.StatusOK)
	}
	metas := decodeData[[]reportseps.ReportMeta](t, list)
	if len(metas) != constants.DefaultIncrementValue || metas[firstIdx].ID != reportID {
		t.Fatalf("list = %+v, want one meta with id %s", metas, reportID)
	}

	for _, format := range reportseps.Formats {
		rec := download(reportID, format)
		if rec.Code != http.StatusOK {
			t.Fatalf(statusFmt, format, rec.Code, http.StatusOK)
		}
		if got, want := rec.Header().Get(constants.HeaderContentType), constants.ReportContentTypes[format]; got != want {
			t.Fatalf(headerFmt, format, constants.HeaderContentType, got, want)
		}
		if got, want := rec.Body.String(), actor+"-"+format; got != want {
			t.Fatalf("%s body = %q, want %q", format, got, want)
		}
	}
}

func listAll(query url.Values) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	reportshandler.ListReports()(rec, httptest.NewRequest(http.MethodGet, routePath+"?"+query.Encode(), nil))
	return rec
}

func TestListAllSetsTotalCountAndRejectsBadFilter(t *testing.T) {
	setRoot(t)
	create(t, createBody(t, actor))
	rec := listAll(url.Values{})
	if rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "list all", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(reportseps.HeaderTotalCount); got != totalOne {
		t.Fatalf(headerFmt, "list all", reportseps.HeaderTotalCount, got, totalOne)
	}
	if metas := decodeData[[]reportseps.ReportMeta](t, rec); len(metas) != constants.DefaultIncrementValue || metas[firstIdx].PlanID != planID {
		t.Fatalf("list all = %+v, want one meta of %s", metas, planID)
	}
	if bad := listAll(url.Values{reportseps.QueryTrigger: {badTrigger}}); bad.Code != http.StatusBadRequest {
		t.Fatalf(statusFmt, "bad trigger", bad.Code, http.StatusBadRequest)
	}
}

func TestCreateReplayReturns200WithStoredMeta(t *testing.T) {
	root := setRoot(t)
	first := decodeData[reportseps.ReportMeta](t, create(t, createBody(t, actor)))
	path := reportsutil.ReportPath(root, planID, reportID)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	rec := create(t, createBody(t, otherActor))
	if rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "replay", rec.Code, http.StatusOK)
	}
	if second := decodeData[reportseps.ReportMeta](t, rec); second != first || second.GeneratedBy != actor {
		t.Fatalf("replay meta = %+v, want %+v", second, first)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("replayed create rewrote the report file")
	}
}

func TestDownloadUnknownFormat400MissingReport404(t *testing.T) {
	setRoot(t)
	create(t, createBody(t, actor))
	if rec := download(reportID, pdfFormat); rec.Code != http.StatusBadRequest {
		t.Fatalf(statusFmt, pdfFormat, rec.Code, http.StatusBadRequest)
	}
	if rec := download(unknownID, reportseps.FormatHTML); rec.Code != http.StatusNotFound {
		t.Fatalf(statusFmt, "missing report", rec.Code, http.StatusNotFound)
	}
	if rec := create(t, "{bad"); rec.Code != http.StatusBadRequest {
		t.Fatalf(statusFmt, "bad json", rec.Code, http.StatusBadRequest)
	}
}

func TestDownloadSetsNoSniffAndSandboxCSP(t *testing.T) {
	setRoot(t)
	create(t, createBody(t, actor))
	rec := download(reportID, reportseps.FormatHTML)
	if rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, downloadName, rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get(constants.HeaderContentTypeOptions); got != constants.ContentTypeOptionsNoSniff {
		t.Fatalf(headerFmt, downloadName, constants.HeaderContentTypeOptions, got, constants.ContentTypeOptionsNoSniff)
	}
	if got := rec.Header().Get(constants.HeaderCSP); got != constants.CSPSandbox {
		t.Fatalf(headerFmt, downloadName, constants.HeaderCSP, got, constants.CSPSandbox)
	}
	if got := rec.Header().Get(constants.HeaderContentDisposition); got != constants.EmptyString {
		t.Fatalf(headerFmt, downloadName, constants.HeaderContentDisposition, got, constants.EmptyString)
	}
}

func TestBodyOverLimit413(t *testing.T) {
	setRoot(t)
	rec := create(t, strings.Repeat("a", overLimit))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(statusFmt, "oversized create", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestLedgerPutGetRoundTripAnd404(t *testing.T) {
	setRoot(t)
	missing := httptest.NewRecorder()
	reportshandler.GetPlanReportLedger()(missing, withID(httptest.NewRequest(http.MethodGet, routePath, nil)))
	if missing.Code != http.StatusNotFound {
		t.Fatalf(statusFmt, "ledger get before put", missing.Code, http.StatusNotFound)
	}

	put := httptest.NewRecorder()
	reportshandler.PutPlanReportLedger()(put, withID(httptest.NewRequest(http.MethodPost, routePath, strings.NewReader(ledgerBody))))
	if put.Code != http.StatusOK {
		t.Fatalf(statusFmt, "ledger put", put.Code, http.StatusOK)
	}

	get := httptest.NewRecorder()
	reportshandler.GetPlanReportLedger()(get, withID(httptest.NewRequest(http.MethodGet, routePath, nil)))
	if get.Code != http.StatusOK {
		t.Fatalf(statusFmt, "ledger get", get.Code, http.StatusOK)
	}
	var env envelope
	if err := json.Unmarshal(get.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if string(env.Data) != ledgerBody {
		t.Fatalf("ledger data = %s, want %s", env.Data, ledgerBody)
	}

	bad := httptest.NewRecorder()
	reportshandler.PutPlanReportLedger()(bad, withID(httptest.NewRequest(http.MethodPost, routePath, strings.NewReader("{bad"))))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf(statusFmt, "ledger put invalid", bad.Code, http.StatusBadRequest)
	}
}

// DeletePlanByID needs a live CR delete (kcore has no client seam and the
// host kubeconfig points at a real cluster), so this pins the store call it
// makes after a 200 from the CR delete: handlers/plans/protection/handler.go.
func TestDeletePlanRemovesItsReports(t *testing.T) {
	root := setRoot(t)
	create(t, createBody(t, actor))
	dir := reportsutil.PlanDir(root, planID)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("plan dir missing before delete: %v", err)
	}
	if err := reportsutil.RemovePlanReports(root, planID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("plan dir still present after delete: %v", err)
	}
}

func TestRequirementsCoverEveryRoute(t *testing.T) {
	requirements := authz.Requirements()
	registered := map[string]bool{}
	for _, route := range routes.InitRoutes(&performance.Optimizer{}) {
		key := route.Method + " " + route.Pattern
		registered[key] = true
		if _, found := requirements[key]; !found {
			t.Errorf("route %q has no authz requirement", key)
		}
	}
	if len(reportRoutes) != reportsCount {
		t.Fatalf("report routes = %d, want %d", len(reportRoutes), reportsCount)
	}
	for _, key := range reportRoutes {
		if !registered[key] {
			t.Errorf("report route %q is not registered", key)
		}
	}
}
