package reports

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/telark/exporter/internal/constants"
	reportsutil "github.com/telark/exporter/internal/utils/reports"
	reportseps "github.com/telark/rest/endpoints/reports"
)

const (
	planC        = "plan-c"
	endA         = "a-end"
	manualB      = "b-manual"
	cancelB      = "b-cancel"
	manualC      = "c-manual"
	atFirst      = "2026-01-01T00:00:00Z"
	atSecond     = "2026-01-02T00:00:00Z"
	atThird      = "2026-01-03T00:00:00Z"
	atFourth     = "2026-01-04T00:00:00Z"
	limitTwo     = "2"
	limitOver    = "5000"
	strayFile    = "stray.json"
	allReports   = 4
	badTime      = "yesterday"
	badLimit     = "many"
	zeroLimit    = "0"
	badTrigger   = "pdf"
	listAllTotal = "total = %d, want %d"
	listAllIDs   = "ids = %v, want %v"
)

func seedPlans(t *testing.T) string {
	t.Helper()
	root := setupRoot(t)
	mustCreate(t, root, request(planA, endA, reportseps.TriggerEnd, atFirst))
	mustCreate(t, root, request(planB, manualB, reportseps.TriggerManual, atSecond))
	mustCreate(t, root, request(planB, cancelB, reportseps.TriggerCancel, atThird))
	mustCreate(t, root, request(planC, manualC, reportseps.TriggerManual, atFourth))
	return root
}

func mustParse(t *testing.T, query url.Values) reportsutil.ListFilter {
	t.Helper()
	filter, err := reportsutil.ParseListFilter(query)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	return filter
}

func mustListAll(t *testing.T, root string, query url.Values) ([]reportseps.ReportMeta, int) {
	t.Helper()
	metas, total, err := reportsutil.ListAll(root, mustParse(t, query))
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	return metas, total
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, filePerm); err != nil {
		t.Fatal(err)
	}
}

func TestListAllSpansPlansNewestFirstWithPlanID(t *testing.T) {
	root := seedPlans(t)
	metas, total := mustListAll(t, root, url.Values{})
	if total != allReports {
		t.Fatalf(listAllTotal, total, allReports)
	}
	want := []string{manualC, cancelB, manualB, endA}
	if got := ids(metas); !slices.Equal(got, want) {
		t.Fatalf(listAllIDs, got, want)
	}
	wantPlans := []string{planC, planB, planB, planA}
	for i, meta := range metas {
		if meta.PlanID != wantPlans[i] {
			t.Fatalf("meta %s planId = %q, want %q", meta.ID, meta.PlanID, wantPlans[i])
		}
	}
}

func TestListAllFilters(t *testing.T) {
	root := seedPlans(t)
	cases := []struct {
		name  string
		query url.Values
		want  []string
	}{
		{"repeated plan ids", url.Values{reportseps.QueryPlanID: {planA, planC}}, []string{manualC, endA}},
		{"comma plan ids", url.Values{reportseps.QueryPlanID: {planA + constants.ReportsListSeparator + planB}}, []string{cancelB, manualB, endA}},
		{"trigger", url.Values{reportseps.QueryTrigger: {reportseps.TriggerManual}}, []string{manualC, manualB}},
		{"from", url.Values{reportseps.QueryFrom: {atThird}}, []string{manualC, cancelB}},
		{"to", url.Values{reportseps.QueryTo: {atSecond}}, []string{manualB, endA}},
		{"range and plan", url.Values{
			reportseps.QueryPlanID: {planB}, reportseps.QueryFrom: {atSecond}, reportseps.QueryTo: {atSecond},
		}, []string{manualB}},
	}
	for _, tc := range cases {
		metas, total := mustListAll(t, root, tc.query)
		if got := ids(metas); !slices.Equal(got, tc.want) || total != len(tc.want) {
			t.Fatalf("%s: ids = %v total = %d, want %v", tc.name, got, total, tc.want)
		}
	}
}

func TestListAllLimitTruncatesButCountsAll(t *testing.T) {
	root := seedPlans(t)
	metas, total := mustListAll(t, root, url.Values{reportseps.QueryLimit: {limitTwo}})
	if total != allReports {
		t.Fatalf(listAllTotal, total, allReports)
	}
	if got, want := ids(metas), []string{manualC, cancelB}; !slices.Equal(got, want) {
		t.Fatalf(listAllIDs, got, want)
	}
	if filter := mustParse(t, url.Values{reportseps.QueryLimit: {limitOver}}); filter.Limit != constants.ReportsListMaxLimit {
		t.Fatalf("limit = %d, want %d", filter.Limit, constants.ReportsListMaxLimit)
	}
	if filter := mustParse(t, url.Values{}); filter.Limit != constants.ReportsListDefaultLimit {
		t.Fatalf("default limit = %d, want %d", filter.Limit, constants.ReportsListDefaultLimit)
	}
}

func TestParseListFilterRejectsBadValues(t *testing.T) {
	for _, query := range []url.Values{
		{reportseps.QueryTrigger: {badTrigger}},
		{reportseps.QueryFrom: {badTime}},
		{reportseps.QueryTo: {badTime}},
		{reportseps.QueryLimit: {badLimit}},
		{reportseps.QueryLimit: {zeroLimit}},
	} {
		if _, err := reportsutil.ParseListFilter(query); err == nil {
			t.Fatalf(wantErrFmt, query.Encode())
		}
	}
}

func TestListAllIgnoresTempAndStrayEntries(t *testing.T) {
	root := seedPlans(t)
	valid, err := json.Marshal(reportsutil.StoredReport{Meta: request(planA, oldTempName, reportseps.TriggerEnd, atFirst).Meta})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(reportsutil.PlanDir(root, planA), constants.ReportsReportsSubdir, oldTempName),
		filepath.Join(root, constants.ReportsPlansSubdir, strayFile),
	} {
		mustWrite(t, path, valid)
	}
	if err := os.MkdirAll(reportsutil.PlanDir(root, livePlan), dirPerm); err != nil {
		t.Fatal(err)
	}
	if metas, total := mustListAll(t, root, url.Values{}); total != allReports || len(metas) != allReports {
		t.Fatalf("got %d metas total %d, want %d", len(metas), total, allReports)
	}
}

func TestListAllEmptyStore(t *testing.T) {
	metas, total := mustListAll(t, setupRoot(t), url.Values{})
	if metas == nil || len(metas) != constants.DefaultInitValue || total != constants.DefaultInitValue {
		t.Fatalf("empty store = %v total %d, want empty non-nil slice", metas, total)
	}
	if _, total := mustListAll(t, seedPlans(t), url.Values{reportseps.QueryPlanID: {orphanNew}}); total != constants.DefaultInitValue {
		t.Fatalf(listAllTotal, total, constants.DefaultInitValue)
	}
}
