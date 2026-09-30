package snapshotapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	expsnap "github.com/telark/telark/services/exporter/internal/exporters/snapshot"
	"github.com/telark/telark/services/exporter/internal/managers/envs"
)

const (
	testAppID      = "app-1"
	otherAppID     = "app-2"
	testNamespace  = "ns"
	testGeneration = "1"
)

func setRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(constants.SnapshotsPathEnv, dir)
	envs.InitSnapshotsPath()
	return dir
}

func manifestBody() map[string]any {
	return map[string]any{
		constants.IDParam:         testAppID,
		constants.ScopeParam:      constants.SnapshotsAppsSubdir,
		constants.NamespaceParam:  testNamespace,
		constants.GenerationParam: float64(constants.DefaultIncrementValue),
		constants.FieldManifest: map[string]any{
			constants.FieldResources: []any{
				map[string]any{constants.FieldManifest: map[string]any{"kind": "Deployment"}},
				map[string]any{constants.FieldManifest: map[string]any{"kind": "Service"}},
			},
		},
	}
}

func createOK(t *testing.T) {
	t.Helper()
	rec := httptest.NewRecorder()
	expsnap.CreateSnapshot(rec, manifestBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("CreateSnapshot code = %d, want 200", rec.Code)
	}
}

func TestCreateSnapshot(t *testing.T) {
	root := setRoot(t)
	createOK(t)
	if _, err := os.Stat(filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, "V1.json")); err != nil {
		t.Errorf("snapshot file not written: %v", err)
	}
}

func TestCreateSnapshotBadRequest(t *testing.T) {
	setRoot(t)
	rec := httptest.NewRecorder()
	// Missing manifest fails request parsing.
	expsnap.CreateSnapshot(rec, map[string]any{constants.IDParam: "a", constants.ScopeParam: constants.SnapshotsAppsSubdir,
		constants.NamespaceParam: testNamespace, constants.GenerationParam: float64(constants.DefaultIncrementValue)})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad create code = %d, want 400", rec.Code)
	}
}

func TestReadSnapshot(t *testing.T) {
	setRoot(t)
	createOK(t)

	rec := httptest.NewRecorder()
	expsnap.ReadSnapshot(rec, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, true)
	if rec.Code != http.StatusOK {
		t.Errorf("ReadSnapshot code = %d, want 200", rec.Code)
	}

	missing := httptest.NewRecorder()
	expsnap.ReadSnapshot(missing, "absent", constants.SnapshotsAppsSubdir, testNamespace, testGeneration, true)
	if missing.Code != http.StatusNotFound {
		t.Errorf("missing read code = %d, want 404", missing.Code)
	}

	// A traversal id fails identity validation with a 400, not a 404.
	bad := httptest.NewRecorder()
	expsnap.ReadSnapshot(bad, "..", constants.SnapshotsAppsSubdir, testNamespace, testGeneration, true)
	if bad.Code != http.StatusBadRequest {
		t.Errorf("invalid id read code = %d, want 400", bad.Code)
	}
}

func TestRemoveSnapshot(t *testing.T) {
	root := setRoot(t)
	createOK(t)

	empty := httptest.NewRecorder()
	expsnap.RemoveSnapshot(empty, testAppID, constants.SnapshotsAppsSubdir, testNamespace, constants.EmptyString)
	if empty.Code != http.StatusBadRequest {
		t.Errorf("delete without generation code = %d, want 400", empty.Code)
	}

	rec := httptest.NewRecorder()
	expsnap.RemoveSnapshot(rec, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration)
	if rec.Code != http.StatusOK {
		t.Fatalf("RemoveSnapshot code = %d, want 200", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, "V1.json")); !os.IsNotExist(err) {
		t.Error("snapshot file survived deletion")
	}

	gone := httptest.NewRecorder()
	expsnap.RemoveSnapshot(gone, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration)
	if gone.Code != http.StatusNotFound {
		t.Errorf("second delete code = %d, want 404", gone.Code)
	}
}

func TestReadSnapshotManifest(t *testing.T) {
	setRoot(t)
	createOK(t)

	jsonRec := httptest.NewRecorder()
	expsnap.ReadSnapshotManifestWithAccept(
		jsonRec, testAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, constants.EmptyString, true,
	)
	if jsonRec.Code != http.StatusOK || jsonRec.Body.Len() == constants.DefaultInitValue {
		t.Errorf("JSON manifest code = %d len = %d", jsonRec.Code, jsonRec.Body.Len())
	}

	yamlRec := httptest.NewRecorder()
	expsnap.ReadSnapshotManifestWithAccept(yamlRec, testAppID, constants.SnapshotsAppsSubdir, testNamespace, "1", "application/yaml", true)
	if yamlRec.Code != http.StatusOK || yamlRec.Body.Len() == constants.DefaultInitValue {
		t.Errorf("YAML manifest code = %d len = %d", yamlRec.Code, yamlRec.Body.Len())
	}
}

func TestReadSnapshotManifestBuildFailure(t *testing.T) {
	setRoot(t)
	// A manifest without a resources list cannot be assembled into a Kubernetes List.
	rec := httptest.NewRecorder()
	body := map[string]any{
		constants.IDParam: otherAppID, constants.ScopeParam: constants.SnapshotsAppsSubdir,
		constants.NamespaceParam: testNamespace, constants.GenerationParam: float64(constants.DefaultIncrementValue),
		constants.FieldManifest: map[string]any{"unexpected": true},
	}
	expsnap.CreateSnapshot(rec, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup create failed: %d", rec.Code)
	}
	man := httptest.NewRecorder()
	expsnap.ReadSnapshotManifestWithAccept(man, otherAppID, constants.SnapshotsAppsSubdir, testNamespace, testGeneration, constants.EmptyString, true)
	if man.Code != http.StatusInternalServerError {
		t.Errorf("unbuildable manifest code = %d, want 500", man.Code)
	}
}

func TestReadSnapshotInfos(t *testing.T) {
	setRoot(t)
	createOK(t)
	rec := httptest.NewRecorder()
	expsnap.ReadSnapshotInfos(rec)
	if rec.Code != http.StatusOK {
		t.Errorf("ReadSnapshotInfos code = %d, want 200", rec.Code)
	}
}
