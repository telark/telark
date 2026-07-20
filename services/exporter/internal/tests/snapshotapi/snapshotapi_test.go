package snapshotapi

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	expsnap "github.com/telark/exporter/internal/exporters/snapshot"
	"github.com/telark/exporter/internal/managers/envs"
)

func setRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()
	return dir
}

func manifestBody(id, namespace string) map[string]any {
	return map[string]any{
		"id":         id,
		"scope":      "apps",
		"namespace":  namespace,
		"generation": float64(1),
		"manifest": map[string]any{
			"resources": []any{
				map[string]any{"manifest": map[string]any{"kind": "Deployment"}},
				map[string]any{"manifest": map[string]any{"kind": "Service"}},
			},
		},
	}
}

func createOK(t *testing.T, id, namespace string) {
	t.Helper()
	rec := httptest.NewRecorder()
	expsnap.CreateSnapshot(rec, manifestBody(id, namespace))
	if rec.Code != 200 {
		t.Fatalf("CreateSnapshot code = %d, want 200", rec.Code)
	}
}

func TestCreateSnapshot(t *testing.T) {
	root := setRoot(t)
	createOK(t, "app-1", "ns")
	if _, err := os.Stat(filepath.Join(root, "apps", "app-1", "ns", "V1.json")); err != nil {
		t.Errorf("snapshot file not written: %v", err)
	}
}

func TestCreateSnapshotBadRequest(t *testing.T) {
	setRoot(t)
	rec := httptest.NewRecorder()
	// Missing manifest fails request parsing.
	expsnap.CreateSnapshot(rec, map[string]any{"id": "a", "scope": "apps", "namespace": "ns", "generation": float64(1)})
	if rec.Code != 400 {
		t.Errorf("bad create code = %d, want 400", rec.Code)
	}
}

func TestReadSnapshot(t *testing.T) {
	setRoot(t)
	createOK(t, "app-1", "ns")

	rec := httptest.NewRecorder()
	expsnap.ReadSnapshot(rec, "app-1", "apps", "ns", "1")
	if rec.Code != 200 {
		t.Errorf("ReadSnapshot code = %d, want 200", rec.Code)
	}

	missing := httptest.NewRecorder()
	expsnap.ReadSnapshot(missing, "absent", "apps", "ns", "1")
	if missing.Code != 404 {
		t.Errorf("missing read code = %d, want 404", missing.Code)
	}

	// A traversal id fails identity validation with a 400, not a 404.
	bad := httptest.NewRecorder()
	expsnap.ReadSnapshot(bad, "..", "apps", "ns", "1")
	if bad.Code != 400 {
		t.Errorf("invalid id read code = %d, want 400", bad.Code)
	}
}

func TestRemoveSnapshot(t *testing.T) {
	root := setRoot(t)
	createOK(t, "app-1", "ns")

	empty := httptest.NewRecorder()
	expsnap.RemoveSnapshot(empty, "app-1", "apps", "ns", "")
	if empty.Code != 400 {
		t.Errorf("delete without generation code = %d, want 400", empty.Code)
	}

	rec := httptest.NewRecorder()
	expsnap.RemoveSnapshot(rec, "app-1", "apps", "ns", "1")
	if rec.Code != 200 {
		t.Fatalf("RemoveSnapshot code = %d, want 200", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "apps", "app-1", "ns", "V1.json")); !os.IsNotExist(err) {
		t.Error("snapshot file survived deletion")
	}

	gone := httptest.NewRecorder()
	expsnap.RemoveSnapshot(gone, "app-1", "apps", "ns", "1")
	if gone.Code != 404 {
		t.Errorf("second delete code = %d, want 404", gone.Code)
	}
}

func TestReadSnapshotManifest(t *testing.T) {
	setRoot(t)
	createOK(t, "app-1", "ns")

	jsonRec := httptest.NewRecorder()
	expsnap.ReadSnapshotManifest(jsonRec, "app-1", "apps", "ns", "1")
	if jsonRec.Code != 200 || jsonRec.Body.Len() == 0 {
		t.Errorf("JSON manifest code = %d len = %d", jsonRec.Code, jsonRec.Body.Len())
	}

	yamlRec := httptest.NewRecorder()
	expsnap.ReadSnapshotManifestWithAccept(yamlRec, "app-1", "apps", "ns", "1", "application/yaml")
	if yamlRec.Code != 200 || yamlRec.Body.Len() == 0 {
		t.Errorf("YAML manifest code = %d len = %d", yamlRec.Code, yamlRec.Body.Len())
	}
}

func TestReadSnapshotManifestBuildFailure(t *testing.T) {
	setRoot(t)
	// A manifest without a resources list cannot be assembled into a Kubernetes List.
	rec := httptest.NewRecorder()
	body := map[string]any{
		"id": "app-2", "scope": "apps", "namespace": "ns", "generation": float64(1),
		"manifest": map[string]any{"unexpected": true},
	}
	expsnap.CreateSnapshot(rec, body)
	if rec.Code != 200 {
		t.Fatalf("setup create failed: %d", rec.Code)
	}
	man := httptest.NewRecorder()
	expsnap.ReadSnapshotManifest(man, "app-2", "apps", "ns", "1")
	if man.Code != 500 {
		t.Errorf("unbuildable manifest code = %d, want 500", man.Code)
	}
}

func TestReadSnapshotInfos(t *testing.T) {
	setRoot(t)
	createOK(t, "app-1", "ns")
	rec := httptest.NewRecorder()
	expsnap.ReadSnapshotInfos(rec)
	if rec.Code != 200 {
		t.Errorf("ReadSnapshotInfos code = %d, want 200", rec.Code)
	}
}
