package reports

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/exporter/internal/constants"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
)

const (
	keptName   = "Payments freeze"
	reportTwo  = "20260102T000000Z-manual-01"
	keptCount  = 2
	noMinAge   = 0
	keptOneFmt = "ListKept = %+v, want one kept plan"
)

func keepTwoReports(t *testing.T, root string) {
	t.Helper()
	mustCreate(t, root, request(planA, reportOne, reportseps.TriggerEnd, generatedAt))
	mustCreate(t, root, request(planA, reportTwo, reportseps.TriggerManual, generatedAt))
	if err := reportsutil.PutLedger(root, planA, []byte(ledgerBody)); err != nil {
		t.Fatalf(errFmt, err)
	}
	if err := reportsutil.Keep(root, planA, keptName); err != nil {
		t.Fatalf(errFmt, err)
	}
}

func mustListKept(t *testing.T, root string) []reportseps.KeptPlanReports {
	t.Helper()
	kept, err := reportsutil.ListKept(root)
	if err != nil {
		t.Fatalf(errFmt, err)
	}
	return kept
}

func TestKeepMovesReportsWhereNoSweepLooks(t *testing.T) {
	root := setupRoot(t)
	keepTwoReports(t, root)

	if exists(reportsutil.PlanDir(root, planA)) {
		t.Fatal("the plan directory is still under plans")
	}
	if exists(filepath.Join(reportsutil.KeptDir(root, planA), constants.ReportsLedgerFile)) {
		t.Fatal("the ledger was kept")
	}
	kept := mustListKept(t, root)
	if len(kept) != constants.DefaultIncrementValue || kept[firstIdx].PlanID != planA || kept[firstIdx].PlanName != keptName {
		t.Fatalf(keptOneFmt, kept)
	}
	got := ids(kept[firstIdx].Reports)
	slices.Sort(got)
	if want := []string{reportOne, reportTwo}; !slices.Equal(got, want) {
		t.Fatalf("kept reports = %v, want %v", got, want)
	}
	for _, format := range reportseps.Formats {
		body, _, err := reportsutil.Load(root, planA, reportOne, format)
		if err != nil || string(body) != files(reportOne)[format] {
			t.Fatalf("Load kept %s = (%q, %v)", format, body, err)
		}
	}

	reportsutil.SweepOrphans(root, map[string]struct{}{planB: {}}, noMinAge)
	if len(mustListKept(t, root)) != constants.DefaultIncrementValue {
		t.Fatal("the orphan sweep removed a kept plan")
	}
	if metas := mustList(t, root, planA); len(metas) != constants.DefaultInitValue {
		t.Fatalf("plan list after keep = %v, want none", metas)
	}
}

func TestKeepWithNoReportsKeepsNothing(t *testing.T) {
	root := setupRoot(t)
	if err := reportsutil.Keep(root, planA, keptName); err != nil {
		t.Fatalf(errFmt, err)
	}
	if exists(filepath.Join(root, constants.ReportsKeptSubdir)) {
		t.Fatal("a keep without reports created the kept directory")
	}
	if kept := mustListKept(t, root); kept == nil || len(kept) != constants.DefaultInitValue {
		t.Fatalf("ListKept = %#v, want an empty list", kept)
	}
	if err := reportsutil.Keep(root, traversalID, keptName); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("traversal keep error = %v, want ErrBadID", err)
	}
}

func TestListKeptShowsAPlanWithoutItsMarker(t *testing.T) {
	root := setupRoot(t)
	keepTwoReports(t, root)
	if err := os.Remove(filepath.Join(reportsutil.KeptDir(root, planA), constants.ReportsKeptMarkerFile)); err != nil {
		t.Fatal(err)
	}
	kept := mustListKept(t, root)
	if len(kept) != constants.DefaultIncrementValue || kept[firstIdx].PlanName != constants.EmptyString ||
		len(kept[firstIdx].Reports) != keptCount {
		t.Fatalf(keptOneFmt, kept)
	}
}

func TestRemoveReportDeletesALiveReportAndLeavesTheRest(t *testing.T) {
	root := setupRoot(t)
	mustCreate(t, root, request(planB, reportOne, reportseps.TriggerEnd, generatedAt))
	mustCreate(t, root, request(planB, reportTwo, reportseps.TriggerManual, generatedAt))

	if err := reportsutil.RemoveReport(root, planB, traversalID); !errors.Is(err, reportsutil.ErrBadID) {
		t.Fatalf("traversal remove error = %v, want ErrBadID", err)
	}
	if err := reportsutil.RemoveReport(root, planB, reportOne); err != nil {
		t.Fatalf(errFmt, err)
	}
	if got := ids(mustList(t, root, planB)); !slices.Equal(got, []string{reportTwo}) {
		t.Fatalf("live plan reports = %v, want [%s]", got, reportTwo)
	}
	if _, _, err := reportsutil.Load(root, planB, reportOne, reportseps.FormatHTML); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted report load error = %v, want ErrNotExist", err)
	}
	if err := reportsutil.RemoveReport(root, planB, reportOne); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second remove error = %v, want ErrNotExist", err)
	}
}

func TestRemoveReportTakesTheKeptDirectoryWithTheLastReport(t *testing.T) {
	root := setupRoot(t)
	keepTwoReports(t, root)

	if err := reportsutil.RemoveReport(root, planA, reportOne); err != nil {
		t.Fatalf(errFmt, err)
	}
	if kept := mustListKept(t, root); len(kept) != constants.DefaultIncrementValue || len(kept[firstIdx].Reports) != constants.DefaultIncrementValue {
		t.Fatalf("after one remove ListKept = %+v, want one plan with one report", kept)
	}
	if err := reportsutil.RemoveReport(root, planA, reportTwo); err != nil {
		t.Fatalf(errFmt, err)
	}
	if exists(reportsutil.KeptDir(root, planA)) {
		t.Fatal("the kept directory outlived its last report")
	}
	if err := reportsutil.RemoveReport(root, planA, reportTwo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("second remove error = %v, want ErrNotExist", err)
	}
}
