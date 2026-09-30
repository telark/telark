package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	snaputil "github.com/telark/telark/services/exporter/internal/utils/snapshot"
)

const payloadFieldSize = 4096

// Performance mode puts several exporter replicas on one ReadWriteMany volume, so
// the same snapshot can be written from several places at once. Every reader must
// still see a whole document.
func TestConcurrentWritesNeverLeaveAPartialSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	const writers = 16
	payload := map[string]any{
		constants.FieldKind: kindDeployment,
		constants.SpecField: strings.Repeat("x", payloadFieldSize),
	}

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for range writers {
		wg.Go(func() {
			if err := snaputil.WriteSnapshotJSON(path, testAppID, payload); err != nil {
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("snapshot unreadable after concurrent writes: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("snapshot is not valid JSON after concurrent writes, a partial write reached readers: %v", err)
	}
	if got[constants.FieldKind] != kindDeployment {
		t.Errorf("snapshot content mangled: kind = %v", got[constants.FieldKind])
	}
}

func TestWriteLeavesNoTempFileBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.json")

	if err := snaputil.WriteSnapshotJSON(path, testAppID, map[string]any{constants.FieldKind: kindService}); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot list snapshot dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Errorf("temp file %s survived a successful write", entry.Name())
		}
	}
	if len(entries) != constants.DefaultIncrementValue {
		t.Errorf("got %d files, want only the snapshot", len(entries))
	}
}
