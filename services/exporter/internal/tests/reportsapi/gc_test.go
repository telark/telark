package reportsapi

import (
	"os"
	"testing"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
	reportsexp "github.com/telark/telark/services/exporter/internal/exporters/reports"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
)

const (
	livePlan  = "live-old"
	orphanNew = "orphan-new"
	orphanOld = "orphan-old"
	oldAge    = 2 * time.Hour
	dirPerm   = 0o750
	panicMsg  = "boom"
)

func mkdirAged(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-age)
	if err := os.Chtimes(dir, past, past); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func seed(t *testing.T) string {
	t.Helper()
	root := setRoot(t)
	mkdirAged(t, reportsutil.PlanDir(root, livePlan), oldAge)
	mkdirAged(t, reportsutil.PlanDir(root, orphanOld), oldAge)
	mkdirAged(t, reportsutil.PlanDir(root, orphanNew), constants.DefaultInitValue)
	return root
}

func TestReportsGCSweepsOnlyUnreferencedOldPlans(t *testing.T) {
	root := seed(t)
	reportsexp.SweepWith(map[string]struct{}{livePlan: {}})
	if exists(reportsutil.PlanDir(root, orphanOld)) {
		t.Fatal("old unreferenced plan dir survived")
	}
	if !exists(reportsutil.PlanDir(root, livePlan)) || !exists(reportsutil.PlanDir(root, orphanNew)) {
		t.Fatal("a live or fresh plan dir was removed")
	}
}

func TestReportsGCSkipsOnEmptyOrFailedList(t *testing.T) {
	root := seed(t)
	reportsexp.SweepWith(map[string]struct{}{})
	reportsexp.SweepWith(nil)
	for _, id := range []string{livePlan, orphanOld, orphanNew} {
		if !exists(reportsutil.PlanDir(root, id)) {
			t.Fatalf("plan dir %s removed on an empty live set", id)
		}
	}
}

func TestReportsGCTickSurvivesPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic escaped the guarded tick: %v", r)
		}
	}()
	reportsexp.RunGuardedWith(func() { panic(panicMsg) })
}
