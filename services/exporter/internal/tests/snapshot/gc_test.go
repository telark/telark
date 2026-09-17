package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	snapshotexp "github.com/telark/exporter/internal/exporters/snapshot"
	"github.com/telark/exporter/internal/managers/envs"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
)

// Two replicas sweep one ReadWriteMany volume, so the sweep is driven by what
// the CRs reference plus an age grace, never by what is on disk alone.
func TestSnapshotGCRemovesOnlyOldUnreferencedFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()

	old := time.Now().Add(-2 * time.Hour)
	write := func(rel string, mtime time.Time) string {
		t.Helper()
		path := filepath.Join(dir, "apps", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		return path
	}
	referenced := write("ref/ns/V1.json", old)
	orphan := write("orphan/ns/V1.json", old)
	young := write("young/ns/V2.json", time.Now())
	leftover := write("crash/ns/V3.json.abc.tmp", old)
	other := write("notes/ns/readme.txt", old)

	orphans, scanned := snaputil.CollectOrphans(dir, map[string]struct{}{referenced: {}}, time.Hour)
	if scanned != 5 {
		t.Errorf("scanned = %d, want 5", scanned)
	}
	want := []string{leftover, orphan}
	slices.Sort(orphans)
	if !slices.Equal(orphans, want) {
		t.Fatalf("orphans = %v, want %v", orphans, want)
	}

	snapshotexp.RemoveSnapshotFiles(orphans)
	for _, path := range []string{referenced, young, other} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: want kept, got %v", path, err)
		}
	}
	for _, path := range want {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: want removed, got %v", path, err)
		}
	}
}
