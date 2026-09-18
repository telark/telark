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

func setupSnapshotsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()
	return dir
}

func writeSnapshotFile(t *testing.T, dir, rel string, mtime time.Time) string {
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

// Creating the namespace dir bumps the snap dir's mtime, so both get stamped.
func mkdirSnapshotDir(t *testing.T, dir, rel string, mtime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, "apps", rel)
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{path, filepath.Dir(path)} {
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func assertRemoved(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: want removed, got %v", path, err)
		}
	}
}

func assertKept(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: want kept, got %v", path, err)
		}
	}
}

// Two replicas sweep one ReadWriteMany volume, so the sweep is driven by what
// the CRs reference plus an age grace, never by what is on disk alone.
func TestSnapshotGCRemovesOnlyOldUnreferencedFiles(t *testing.T) {
	dir := setupSnapshotsDir(t)
	old := time.Now().Add(-2 * time.Hour)
	referenced := writeSnapshotFile(t, dir, "ref/ns/V1.json", old)
	orphan := writeSnapshotFile(t, dir, "orphan/ns/V1.json", old)
	young := writeSnapshotFile(t, dir, "young/ns/V2.json", time.Now())
	leftover := writeSnapshotFile(t, dir, "crash/ns/V3.json.abc.tmp", old)
	other := writeSnapshotFile(t, dir, "notes/ns/readme.txt", old)

	orphans, emptyDirs, scanned := snaputil.CollectOrphans(dir, map[string]struct{}{referenced: {}}, time.Hour)
	if scanned != 5 || len(emptyDirs) != 0 {
		t.Errorf("scanned = %d, emptyDirs = %v, want 5 and none", scanned, emptyDirs)
	}
	want := []string{leftover, orphan}
	slices.Sort(orphans)
	if !slices.Equal(orphans, want) {
		t.Fatalf("orphans = %v, want %v", orphans, want)
	}

	snapshotexp.RemoveSnapshotFiles(orphans)
	assertKept(t, []string{referenced, young, other})
	assertRemoved(t, want)
}

// Every leftover directory costs EFS a metadata op on each stats walk, so the
// sweep prunes them too: never the root or a scope dir, only past the age gate.
func TestSnapshotGCRemovesEmptySnapshotDirs(t *testing.T) {
	dir := setupSnapshotsDir(t)
	old := time.Now().Add(-2 * time.Hour)
	referenced := writeSnapshotFile(t, dir, "ref/ns/V1.json", old)
	orphan := writeSnapshotFile(t, dir, "orphan/ns/V1.json", old)
	stale := mkdirSnapshotDir(t, dir, "stale/ns", old)
	fresh := mkdirSnapshotDir(t, dir, "fresh/ns", time.Now())

	orphans, emptyDirs, _ := snaputil.CollectOrphans(dir, map[string]struct{}{referenced: {}}, time.Hour)
	if !slices.Equal(orphans, []string{orphan}) {
		t.Fatalf("orphans = %v, want %v", orphans, []string{orphan})
	}
	wantDirs := []string{stale, filepath.Dir(stale)}
	if !slices.Equal(emptyDirs, wantDirs) {
		t.Fatalf("emptyDirs = %v, want %v", emptyDirs, wantDirs)
	}

	removed := snapshotexp.RemoveSnapshotDirs(emptyDirs)
	removed += snapshotexp.RemoveSnapshotFiles(orphans)
	if removed != 4 {
		t.Errorf("removed dirs = %d, want 4", removed)
	}
	assertRemoved(t, []string{stale, filepath.Dir(stale), filepath.Dir(orphan), filepath.Dir(filepath.Dir(orphan))})
	assertKept(t, []string{dir, filepath.Join(dir, "apps"), fresh, filepath.Dir(fresh), referenced})
}
