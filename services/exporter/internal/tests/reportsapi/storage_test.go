package reportsapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	reportshandler "github.com/telark/telark/services/exporter/internal/handlers/reports"
	"github.com/telark/telark/services/exporter/internal/managers/envs"
	reportsutil "github.com/telark/telark/services/exporter/internal/utils/reports"
)

const (
	storagePath      = "/reports/storage"
	storagePVC       = "reports-claim"
	storageNamespace = "ops"
	missingDir       = "absent"
	noReports        = 0
	oneReport        = 1
	noBytes          = 0
)

type storageSpace struct {
	Bytes   uint64   `json:"bytes"`
	Percent *float64 `json:"percent"`
}

type storageInfo struct {
	Total        storageSpace `json:"totalPVCSpace"`
	Consumed     storageSpace `json:"consumedSpace"`
	Available    storageSpace `json:"availableSpace"`
	Reports      int          `json:"totalReports"`
	PVCName      string       `json:"pvcName"`
	PVCNamespace string       `json:"pvcNamespace"`
}

func readStorage(t *testing.T) storageInfo {
	t.Helper()
	rec := httptest.NewRecorder()
	reportshandler.GetReportsStorage()(rec, httptest.NewRequest(http.MethodGet, storagePath, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "storage", rec.Code, http.StatusOK)
	}
	return decodeData[storageInfo](t, rec)
}

func TestReportsStorageCountsReportsAndNamesTheClaim(t *testing.T) {
	t.Setenv(constants.ReportsPVCNameEnv, storagePVC)
	t.Setenv(constants.ReportsPVCNamespaceEnv, storageNamespace)
	setRoot(t)
	if empty := readStorage(t); empty.Reports != noReports || empty.Consumed.Bytes != noBytes {
		t.Fatalf("empty volume = %+v, want no reports and no bytes", empty)
	}
	if rec := create(t, createBody(t, actor)); rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "create", rec.Code, http.StatusOK)
	}
	got := readStorage(t)
	if got.Reports != oneReport || got.Consumed.Bytes == noBytes {
		t.Fatalf("after one report = %+v, want 1 report and some bytes", got)
	}
	// No cluster here: the capacity reads as zero and the percentages are left out.
	if got.Total.Bytes != noBytes || got.Available.Bytes != noBytes || got.Consumed.Percent != nil {
		t.Fatalf("unknown capacity = %+v, want zeros without percentages", got)
	}
	if got.PVCName != storagePVC || got.PVCNamespace != storageNamespace {
		t.Fatalf("claim = %s/%s, want %s/%s", got.PVCNamespace, got.PVCName, storageNamespace, storagePVC)
	}
}

func TestReportsStorageAnswersWhenTheVolumeIsMissing(t *testing.T) {
	t.Setenv(constants.ReportsPathEnv, filepath.Join(t.TempDir(), missingDir))
	envs.InitReportsPath()
	if got := readStorage(t); got.Reports != noReports || got.Consumed.Bytes != noBytes {
		t.Fatalf("missing volume = %+v, want zeros", got)
	}
}

func TestReportsUsageCountsKeptReports(t *testing.T) {
	root := setRoot(t)
	if rec := create(t, createBody(t, actor)); rec.Code != http.StatusOK {
		t.Fatalf(statusFmt, "create", rec.Code, http.StatusOK)
	}
	if err := reportsutil.Keep(root, planID, keptName); err != nil {
		t.Fatalf("keep: %v", err)
	}
	used, count, err := reportsutil.Usage(root)
	if err != nil || count != oneReport || used == noBytes {
		t.Fatalf("usage = %d bytes, %d reports, %v; want some bytes and 1 report", used, count, err)
	}
	if _, _, err := reportsutil.Usage(filepath.Join(root, missingDir)); err == nil {
		t.Fatal("usage of a missing root returned no error")
	}
}
