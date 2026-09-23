package reports

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/managers/envs"
	reportsutil "github.com/telark/exporter/internal/utils/reports"
	reportseps "github.com/telark/rest/endpoints/reports"
)

const (
	planA          = "plan-a"
	planB          = "plan-b"
	reportOne      = "20260101T000000Z-end"
	actor          = "alice"
	otherActor     = "bob"
	generatedAt    = "2026-01-01T00:00:00Z"
	manualCount    = 12
	concurrency    = 8
	oldAge         = 2 * time.Hour
	sweepMinAge    = time.Hour
	livePlan       = "live-new"
	orphanNew      = "orphan-new"
	orphanOld      = "orphan-old"
	oldTempName    = "x.json.123.tmp"
	freshTempName  = "y.json.456.tmp"
	bigBodyBytes   = 20 << 20
	maxMetaRead    = 64 << 10
	tempGlob       = "*.tmp"
	metaPrefix     = `{"meta":`
	dirPerm        = 0o750
	filePerm       = 0o600
	ledgerBody     = `{"planId":"plan-a","violations":[]}`
	badLedgerBody  = `{"planId":`
	errFmt         = "unexpected error: %v"
	wantErrFmt     = "want error, got nil for %s"
	manualIDLayout = "20260101T000000Z-manual-%02d"
	manualAtLayout = "2026-01-01T00:%02d:00Z"
	traversalID    = ".."
	firstIdx       = 0
	removeTwice    = 2
	sweepScanned   = 3
)

func setupRoot(t *testing.T) string {
	t.Helper()
	t.Setenv(constants.ReportsPathEnv, t.TempDir())
	return envs.InitReportsPath()
}

func files(tag string) map[string]string {
	out := make(map[string]string, len(reportseps.Formats))
	for _, f := range reportseps.Formats {
		out[f] = tag + "-" + f
	}
	return out
}

func request(planID, reportID, trigger, at string) reportseps.CreatePlanReportRequest {
	return reportseps.CreatePlanReportRequest{
		Meta: reportseps.ReportMeta{
			ID: reportID, PlanID: planID, Trigger: trigger, GeneratedAt: at, GeneratedBy: actor,
		},
		Files: files(reportID),
	}
}

func mustCreate(t *testing.T, root string, req reportseps.CreatePlanReportRequest) reportseps.ReportMeta {
	t.Helper()
	meta, replayed, err := reportsutil.Create(root, req)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if replayed {
		t.Fatalf("first create of %s reported replayed", req.Meta.ID)
	}
	return meta
}

func mustList(t *testing.T, root, planID string) []reportseps.ReportMeta {
	t.Helper()
	metas, err := reportsutil.List(root, planID)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	return metas
}

func ids(metas []reportseps.ReportMeta) []string {
	out := make([]string, firstIdx, len(metas))
	for _, m := range metas {
		out = append(out, m.ID)
	}
	return out
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	return info
}

func mkdirOld(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-age)
	if err := os.Chtimes(dir, past, past); err != nil {
		t.Fatal(err)
	}
}

func writeAged(t *testing.T, path string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(metaPrefix), filePerm); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-age)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCreateWritesOneFileAndListSeesIt(t *testing.T) {
	root := setupRoot(t)
	req := request(planA, reportOne, reportseps.TriggerEnd, generatedAt)

	meta := mustCreate(t, root, req)
	if meta != req.Meta {
		t.Fatalf("meta = %+v, want %+v", meta, req.Meta)
	}
	entries, err := os.ReadDir(filepath.Dir(reportsutil.ReportPath(root, planA, reportOne)))
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if len(entries) != constants.DefaultIncrementValue || entries[firstIdx].Name() != reportOne+constants.ReportFileExtension {
		t.Fatalf("reports dir entries = %v, want exactly %s.json", entries, reportOne)
	}
	metas := mustList(t, root, planA)
	if len(metas) != constants.DefaultIncrementValue || metas[firstIdx] != req.Meta {
		t.Fatalf("List = %+v, want [%+v]", metas, req.Meta)
	}
}

func TestCreateReplaySameIDReturnsStoredMetaWithoutRewrite(t *testing.T) {
	root := setupRoot(t)
	first := request(planA, reportOne, reportseps.TriggerEnd, generatedAt)
	mustCreate(t, root, first)
	path := reportsutil.ReportPath(root, planA, reportOne)
	before := mustStat(t, path)

	second := first
	second.Meta.GeneratedBy = otherActor
	second.Files = files(otherActor)
	meta, replayed, err := reportsutil.Create(root, second)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if !replayed || meta != first.Meta {
		t.Fatalf("replay = (%+v, %v), want (%+v, true)", meta, replayed, first.Meta)
	}
	if after := mustStat(t, path); !after.ModTime().Equal(before.ModTime()) || after.Size() != before.Size() {
		t.Fatal("replayed create rewrote the report file")
	}
}

func TestCreateRejectsTraversalIDsAndUnknownFormat(t *testing.T) {
	root := setupRoot(t)
	badPlan := request(traversalID, reportOne, reportseps.TriggerEnd, generatedAt)
	badReport := request(planA, "a/b", reportseps.TriggerEnd, generatedAt)
	missingFormat := request(planA, reportOne, reportseps.TriggerEnd, generatedAt)
	delete(missingFormat.Files, reportseps.FormatCSV)
	unknownFormat := request(planA, reportOne, reportseps.TriggerEnd, generatedAt)
	delete(unknownFormat.Files, reportseps.FormatCSV)
	unknownFormat.Files["exe"] = "x"

	for name, req := range map[string]reportseps.CreatePlanReportRequest{
		"plan ..": badPlan, "report a/b": badReport, "missing csv": missingFormat, "unknown exe": unknownFormat,
	} {
		if _, _, err := reportsutil.Create(root, req); err == nil {
			t.Fatalf(wantErrFmt, name)
		}
	}
	if _, _, err := reportsutil.Create(root, badPlan); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("plan .. error = %v, want ErrBadID", err)
	}
	if _, _, err := reportsutil.Create(root, unknownFormat); !errors.Is(err, reportsutil.ErrBadFormat) {
		t.Fatalf("unknown exe error = %v, want ErrBadFormat", err)
	}
	if exists(filepath.Join(root, constants.ReportsPlansSubdir)) || exists(reportsutil.PlanDir(root, planA)) {
		t.Fatal("a rejected create left a directory behind")
	}
}

func TestPruneKeepsNewestManualAndNeverPrunesBoundary(t *testing.T) {
	root := setupRoot(t)
	endID := "20260101T000000Z-" + reportseps.TriggerEnd
	cancelID := "20260101T000000Z-" + reportseps.TriggerCancel
	mustCreate(t, root, request(planA, endID, reportseps.TriggerEnd, generatedAt))
	mustCreate(t, root, request(planA, cancelID, reportseps.TriggerCancel, generatedAt))
	want := []string{endID, cancelID}
	for i := constants.DefaultInitValue; i < manualCount; i++ {
		id := fmt.Sprintf(manualIDLayout, i)
		mustCreate(t, root, request(planA, id, reportseps.TriggerManual, fmt.Sprintf(manualAtLayout, i+constants.DefaultIncrementValue)))
		if i >= manualCount-constants.ReportsMaxPerPlan {
			want = append(want, id)
		}
	}

	got := ids(mustList(t, root, planA))
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("after prune ids = %v, want %v", got, want)
	}
}

func TestLoadServesFormatBytesAndRefusesOutsideRoot(t *testing.T) {
	root := setupRoot(t)
	req := request(planA, reportOne, reportseps.TriggerEnd, generatedAt)
	mustCreate(t, root, req)

	for _, format := range reportseps.Formats {
		body, contentType, err := reportsutil.Load(root, planA, reportOne, format)
		if err != nil {
			t.Fatalf(errFmt, err)
		}
		if string(body) != req.Files[format] || contentType != constants.ReportContentTypes[format] {
			t.Fatalf("Load %s = (%q, %q), want (%q, %q)", format, body, contentType, req.Files[format], constants.ReportContentTypes[format])
		}
	}
	if _, _, err := reportsutil.Load(root, planA, "../"+reportOne, reportseps.FormatHTML); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("traversal error = %v, want ErrBadID", err)
	}
	if _, _, err := reportsutil.Load(root, planA, reportOne, "exe"); !errors.Is(err, reportsutil.ErrBadFormat) {
		t.Fatalf("unknown format error = %v, want ErrBadFormat", err)
	}
	if _, _, err := reportsutil.Load(root, planA, "missing", reportseps.FormatHTML); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing report error = %v, want ErrNotExist", err)
	}
}

func TestLedgerPutGetRoundTripAndRejectsInvalidJSON(t *testing.T) {
	root := setupRoot(t)
	if err := reportsutil.PutLedger(root, planA, []byte(ledgerBody)); err != nil {
		t.Fatalf(errFmt, err)
	}
	if !exists(reportsutil.LedgerPath(root, planA)) {
		t.Fatal("ledger file missing at LedgerPath")
	}
	got, err := reportsutil.GetLedger(root, planA)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if string(got) != ledgerBody {
		t.Fatalf("GetLedger = %s, want %s", got, ledgerBody)
	}
	if err := reportsutil.PutLedger(root, planA, []byte(badLedgerBody)); !errors.Is(err, reportsutil.ErrInvalidLedger) {
		t.Fatalf("invalid ledger error = %v, want ErrInvalidLedger", err)
	}
	if err := reportsutil.PutLedger(root, traversalID, []byte(ledgerBody)); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("traversal ledger error = %v, want ErrBadID", err)
	}
	if _, err := reportsutil.GetLedger(root, planB); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ledger error = %v, want ErrNotExist", err)
	}
}

func TestRemovePlanReportsRemovesOnlyThatPlanAndIsIdempotent(t *testing.T) {
	root := setupRoot(t)
	mustCreate(t, root, request(planA, reportOne, reportseps.TriggerEnd, generatedAt))
	mustCreate(t, root, request(planB, reportOne, reportseps.TriggerEnd, generatedAt))

	for range removeTwice {
		if err := reportsutil.RemovePlanReports(root, planA); err != nil {
			t.Fatalf(errFmt, err)
		}
	}
	if exists(reportsutil.PlanDir(root, planA)) {
		t.Fatal("plan-a directory still exists")
	}
	if got := ids(mustList(t, root, planB)); !slices.Equal(got, []string{reportOne}) {
		t.Fatalf("plan-b reports = %v, want [%s]", got, reportOne)
	}
	if err := reportsutil.RemovePlanReports(root, traversalID); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("traversal remove error = %v, want ErrBadID", err)
	}
}

func TestSweepOrphansRemovesOnlyOldUnreferencedPlanDirsAndOldTemps(t *testing.T) {
	root := setupRoot(t)
	liveReports := filepath.Join(reportsutil.PlanDir(root, livePlan), constants.ReportsReportsSubdir)
	writeAged(t, filepath.Join(liveReports, oldTempName), oldAge)
	writeAged(t, filepath.Join(liveReports, freshTempName), constants.DefaultInitValue)
	mkdirOld(t, reportsutil.PlanDir(root, orphanNew), constants.DefaultInitValue)
	mkdirOld(t, reportsutil.PlanDir(root, orphanOld), oldAge)

	removed, temps, scanned := reportsutil.SweepOrphans(root, map[string]struct{}{livePlan: {}}, sweepMinAge)
	if removed != constants.DefaultIncrementValue || temps != constants.DefaultIncrementValue || scanned != sweepScanned {
		t.Fatalf("SweepOrphans = (%d, %d, %d), want (1, 1, 3)", removed, temps, scanned)
	}
	if exists(reportsutil.PlanDir(root, orphanOld)) {
		t.Fatal("old orphan plan dir survived")
	}
	if !exists(reportsutil.PlanDir(root, orphanNew)) || !exists(reportsutil.PlanDir(root, livePlan)) {
		t.Fatal("a fresh orphan or the live plan dir was removed")
	}
	if exists(filepath.Join(liveReports, oldTempName)) || !exists(filepath.Join(liveReports, freshTempName)) {
		t.Fatal("temp sweep removed the wrong file")
	}
}

func TestConcurrentCreatesLeaveNoTemp(t *testing.T) {
	root := setupRoot(t)
	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Go(func() {
			id := fmt.Sprintf(manualIDLayout, i)
			if _, _, err := reportsutil.Create(root, request(planA, id, reportseps.TriggerManual, generatedAt)); err != nil {
				t.Errorf(errFmt, err)
			}
		})
	}
	wg.Wait()

	reportsDir := filepath.Dir(reportsutil.ReportPath(root, planA, reportOne))
	if left, _ := filepath.Glob(filepath.Join(reportsDir, tempGlob)); len(left) != constants.DefaultInitValue {
		t.Fatalf("temp files left behind: %v", left)
	}
	if got := mustList(t, root, planA); len(got) != concurrency {
		t.Fatalf("List = %d reports, want %d", len(got), concurrency)
	}
}

type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

func TestReadMetaStopsBeforeFiles(t *testing.T) {
	want := request(planA, reportOne, reportseps.TriggerEnd, generatedAt).Meta
	stored := reportsutil.StoredReport{Meta: want, Files: map[string]string{reportseps.FormatHTML: strings.Repeat("x", bigBodyBytes)}}
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	counter := &countingReader{r: bytes.NewReader(raw)}

	got, err := reportsutil.ReadMeta(counter)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if got != want {
		t.Fatalf("ReadMeta = %+v, want %+v", got, want)
	}
	if counter.n >= maxMetaRead {
		t.Fatalf("ReadMeta consumed %d bytes, want fewer than %d", counter.n, maxMetaRead)
	}
	if _, err := reportsutil.ReadMeta(strings.NewReader(`{"files":{},"meta":{}}`)); err == nil {
		t.Fatalf(wantErrFmt, "files before meta")
	}
}

func TestStoredReportMetaIsFirstMember(t *testing.T) {
	raw, err := json.Marshal(reportsutil.StoredReport{Meta: reportseps.ReportMeta{ID: reportOne}, Files: files(reportOne)})
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	if !bytes.HasPrefix(raw, []byte(metaPrefix)) {
		t.Fatalf("StoredReport JSON starts with %q, want %q", raw[:len(metaPrefix)], metaPrefix)
	}
}
