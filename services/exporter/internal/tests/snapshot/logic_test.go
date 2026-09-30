package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/managers/envs"
	"github.com/telark/exporter/internal/utils/artifact"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
)

const (
	bytesPerKB = 1024
	halfKB     = 512
	twoKB      = 2 * bytesPerKB
	threeMB    = 3 * bytesPerKB * bytesPerKB
	oneGB      = bytesPerKB * bytesPerKB * bytesPerKB
	strayBytes = 10

	filePerm = 0o600
	dirPerm  = 0o750

	testAppID      = "app-1"
	testID         = "id"
	testNamespace  = "ns"
	snapshotRoot   = "/s"
	basePath       = "/base"
	caseValid      = "valid"
	valueX         = "x"
	kindService    = "Service"
	kindDeployment = "Deployment"

	fieldAnnotations           = "annotations"
	fieldInternalTrafficPolicy = "internalTrafficPolicy"

	// Snapshot generations written by the fixtures; only their ordering matters.
	gen1 = 1
	gen2 = 2
	gen3 = 3
	gen4 = 4
	gen5 = 5
	gen6 = 6
	gen7 = 7

	retainedVersions = 3

	wantOrderedItems  = 2
	wantSanitizedKept = 3
	fractionalGen     = 2.5

	wantSnapshots        = 2
	wantSnapshotBytes    = 4
	wantRefreshedCount   = 3
	statsWalkDeadline    = 5 * time.Second
	statsWalkPollBackoff = 10 * time.Millisecond
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{"bytes", halfKB, "512 B"},
		{"kilobytes", twoKB, "2.00 KB"},
		{"megabytes", threeMB, "3.00 MB"},
		{"gigabytes", oneGB, "1.00 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snaputil.FormatBytes(tt.bytes); got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormattedFileSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.json")
	if err := os.WriteFile(path, make([]byte, twoKB), filePerm); err != nil {
		t.Fatal(err)
	}
	if got := snaputil.FormattedFileSize(path, testID); got != "2.00 KB" {
		t.Errorf("FormattedFileSize existing = %q, want 2.00 KB", got)
	}
	if got := snaputil.FormattedFileSize(filepath.Join(dir, "missing"), testID); got != "unknown" {
		t.Errorf("FormattedFileSize missing = %q, want unknown", got)
	}
}

func TestBuildKubernetesItems(t *testing.T) {
	valid := map[string]any{
		constants.FieldManifest: map[string]any{
			constants.FieldResources: []any{
				map[string]any{constants.FieldManifest: map[string]any{constants.FieldKind: kindDeployment}},
				map[string]any{constants.FieldManifest: map[string]any{constants.FieldKind: kindService}},
			},
		},
	}
	items, ok := snaputil.BuildKubernetesItems(valid)
	if !ok || len(items) != wantOrderedItems {
		t.Fatalf("BuildKubernetesItems ok=%v len=%d, want true/2", ok, len(items))
	}
	// Service applies before Deployment, so the sort must reorder them.
	first := items[constants.DefaultInitValue][constants.FieldKind]
	second := items[constants.DefaultIncrementValue][constants.FieldKind]
	if first != kindService || second != kindDeployment {
		t.Errorf("items not ordered by apply weight: %v, %v", first, second)
	}

	bad := []struct {
		name string
		in   map[string]any
	}{
		{"no manifest key", map[string]any{}},
		{"manifest not map", map[string]any{constants.FieldManifest: valueX}},
		{"resources not slice", map[string]any{constants.FieldManifest: map[string]any{constants.FieldResources: valueX}}},
		{"resource not map", map[string]any{constants.FieldManifest: map[string]any{constants.FieldResources: []any{valueX}}}},
		{
			"resource without inner manifest",
			map[string]any{constants.FieldManifest: map[string]any{constants.FieldResources: []any{map[string]any{}}}},
		},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := snaputil.BuildKubernetesItems(tt.in); ok {
				t.Errorf("BuildKubernetesItems(%v) ok=true, want false", tt.in)
			}
		})
	}
}

func TestIsWithinBase(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		base       string
		wantWithin bool
	}{
		{"child path", "/base/sub/x.json", basePath, true},
		{"equal path", basePath, basePath, false},
		{"sibling prefix no separator", "/basex", basePath, false},
		{"outside", "/other/x", basePath, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := artifact.IsWithinBase(tt.target, tt.base); got != tt.wantWithin {
				t.Errorf("IsWithinBase(%q,%q) = %v, want %v", tt.target, tt.base, got, tt.wantWithin)
			}
		})
	}
}

func TestBuildSnapshotDirAndPath(t *testing.T) {
	nsDir := snaputil.BuildSnapshotDir(snapshotRoot, constants.SnapshotsAppsSubdir, testID, testNamespace, true)
	if nsDir != "/s/apps/id/ns" {
		t.Errorf("namespaced dir = %q", nsDir)
	}
	clusterDir := snaputil.BuildSnapshotDir(snapshotRoot, constants.SnapshotsAppsSubdir, testID, testNamespace, false)
	if clusterDir != "/s/apps/id" {
		t.Errorf("cluster-scoped dir = %q", clusterDir)
	}
	nsPath := snaputil.BuildSnapshotPath(snapshotRoot, constants.SnapshotsAppsSubdir, testID, testNamespace, gen3, true)
	if nsPath != "/s/apps/id/ns/V3.json" {
		t.Errorf("namespaced path = %q", nsPath)
	}
	clusterPath := snaputil.BuildSnapshotPath(snapshotRoot, constants.SnapshotsAppsSubdir, testID, testNamespace, gen3, false)
	if clusterPath != "/s/apps/id/V3.json" {
		t.Errorf("cluster-scoped path = %q", clusterPath)
	}
}

func TestValidateSnapshotIdentity(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		scope   string
		wantErr bool
	}{
		{caseValid, testAppID, constants.SnapshotsAppsSubdir, false},
		{"empty id", constants.EmptyString, constants.SnapshotsAppsSubdir, true},
		{"empty scope", testAppID, constants.EmptyString, true},
		{"traversal id", "..", constants.SnapshotsAppsSubdir, true},
		{"path in id", "a/b", constants.SnapshotsAppsSubdir, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := snaputil.ValidateSnapshotIdentity(tt.id, tt.scope)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSnapshotIdentity(%q,%q) err=%v, wantErr=%v", tt.id, tt.scope, err, tt.wantErr)
			}
		})
	}
}

func TestParseCreateSnapshotRequest(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{
			constants.IDParam: testAppID, constants.ScopeParam: constants.SnapshotsAppsSubdir,
			constants.NamespaceParam: testNamespace, constants.GenerationParam: float64(gen1),
			constants.FieldManifest: map[string]any{valueX: constants.DefaultIncrementValue},
		}
	}
	t.Run(caseValid, func(t *testing.T) {
		got, err := snaputil.ParseCreateSnapshotRequest(base())
		if err != nil || got.ID != testAppID || got.Scope != constants.SnapshotsAppsSubdir || got.Namespace != testNamespace {
			t.Fatalf("ParseCreateSnapshotRequest = %+v, err %v", got, err)
		}
	})
	mutate := map[string]func(m map[string]any){
		"missing id":            func(m map[string]any) { delete(m, constants.IDParam) },
		"missing scope":         func(m map[string]any) { delete(m, constants.ScopeParam) },
		"unregistered scope":    func(m map[string]any) { m[constants.ScopeParam] = "nope" },
		"namespaced without ns": func(m map[string]any) { delete(m, constants.NamespaceParam) },
		"missing generation":    func(m map[string]any) { delete(m, constants.GenerationParam) },
		"missing manifest":      func(m map[string]any) { delete(m, constants.FieldManifest) },
	}
	for name, mut := range mutate {
		t.Run(name, func(t *testing.T) {
			m := base()
			mut(m)
			if _, err := snaputil.ParseCreateSnapshotRequest(m); err == nil {
				t.Errorf("%s: expected error", name)
			}
		})
	}
}

func TestRegisteredScopesText(t *testing.T) {
	if got := snaputil.RegisteredScopesText(envs.GetSnapshotScopes()); got != constants.SnapshotsAppsSubdir {
		t.Errorf("RegisteredScopesText = %q, want apps", got)
	}
}

func TestSnapshotNotFoundMessage(t *testing.T) {
	if snaputil.SnapshotNotFoundMessage(testID, constants.SnapshotsAppsSubdir, testNamespace, constants.EmptyString) == constants.EmptyString {
		t.Error("empty message for latest")
	}
	if snaputil.SnapshotNotFoundMessage(testID, constants.SnapshotsAppsSubdir, testNamespace, "3") == constants.EmptyString {
		t.Error("empty message for explicit generation")
	}
}

func TestContentDispositionFilename(t *testing.T) {
	cases := map[string]string{
		constants.SnapshotRollbackFilenameSuffix:     `attachment; filename="app-1-G3-rollback.json"`,
		constants.SnapshotRollbackYAMLFilenameSuffix: `attachment; filename="app-1-G3-rollback.yaml"`,
	}
	for suffix, want := range cases {
		if got := snaputil.ContentDispositionFilename(testAppID, gen3, suffix); got != want {
			t.Errorf("ContentDispositionFilename = %q, want %q", got, want)
		}
	}
}

func TestLoadSnapshotData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(constants.SnapshotsPathEnv, dir)
	envs.InitSnapshotsPath()

	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"kind":"Service"}`), filePerm); err != nil {
		t.Fatal(err)
	}
	data, err := snaputil.LoadSnapshotData(good)
	if err != nil || data[constants.FieldKind] != kindService {
		t.Fatalf("LoadSnapshotData good = %v, err %v", data, err)
	}

	if _, err := snaputil.LoadSnapshotData("/etc/hosts"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("outside base err = %v, want ErrNotExist", err)
	}
	if _, err := snaputil.LoadSnapshotData(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file in base: expected error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), filePerm); err != nil {
		t.Fatal(err)
	}
	if _, err := snaputil.LoadSnapshotData(bad); err == nil {
		t.Error("invalid json: expected error")
	}
}

func writeGen(t *testing.T, dir string, gens ...int) {
	t.Helper()
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		t.Fatal(err)
	}
	for _, g := range gens {
		p := filepath.Join(dir, "V"+strconv.Itoa(g)+".json")
		if err := os.WriteFile(p, []byte("{}"), filePerm); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveSnapshotPathNamespaced(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace)
	writeGen(t, nsDir, gen1, gen3, gen2)

	t.Run("latest", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, constants.EmptyString, true)
		if err != nil || target.Generation != gen3 {
			t.Fatalf("latest = %+v, err %v", target, err)
		}
	})
	t.Run("explicit found", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, "2", true)
		if err != nil || target.Generation != gen2 {
			t.Fatalf("explicit = %+v, err %v", target, err)
		}
	})
	t.Run("explicit missing", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, "9", true); err == nil {
			t.Error("missing generation: expected error")
		}
	})
	t.Run("invalid generation", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, "abc", true); err == nil {
			t.Error("invalid generation: expected error")
		}
	})
}

func TestResolveSnapshotPathAcrossNamespaces(t *testing.T) {
	root := t.TempDir()
	writeGen(t, filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, "ns1"), gen1)
	writeGen(t, filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, "ns2"), gen4)

	t.Run("latest picks highest across namespaces", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, constants.EmptyString, "latest", true)
		if err != nil || target.Generation != gen4 || target.Namespace != "ns2" {
			t.Fatalf("across latest = %+v, err %v", target, err)
		}
	})
	t.Run("explicit across namespaces", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, constants.EmptyString, "1", true)
		if err != nil || target.Generation != gen1 || target.Namespace != "ns1" {
			t.Fatalf("across explicit = %+v, err %v", target, err)
		}
	})
	t.Run("base dir missing", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, "absent", constants.EmptyString, "latest", true); err == nil {
			t.Error("missing base: expected error")
		}
	})
}

func TestResolveSnapshotPathClusterScoped(t *testing.T) {
	root := t.TempDir()
	writeGen(t, filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID), gen5, gen7)
	target, err := snaputil.ResolveSnapshotPath(root, constants.SnapshotsAppsSubdir, testAppID, constants.EmptyString, constants.EmptyString, false)
	if err != nil || target.Generation != gen7 {
		t.Fatalf("cluster-scoped latest = %+v, err %v", target, err)
	}
}

func TestApplyRetentionPolicy(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace)
	writeGen(t, nsDir, gen1, gen2, gen3, gen4, gen5, gen6)
	// A stray non-version file must never be counted or deleted.
	stray := filepath.Join(nsDir, "notes.txt")
	if err := os.WriteFile(stray, []byte(valueX), filePerm); err != nil {
		t.Fatal(err)
	}

	if err := snaputil.ApplyRetentionPolicy(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, true, retainedVersions); err != nil {
		t.Fatalf("ApplyRetentionPolicy: %v", err)
	}
	for _, g := range []int{gen1, gen2, gen3} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+strconv.Itoa(g)+".json")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("V%d.json should have been pruned", g)
		}
	}
	for _, g := range []int{gen4, gen5, gen6} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+strconv.Itoa(g)+".json")); err != nil {
			t.Errorf("V%d.json should have been kept: %v", g, err)
		}
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("stray file was deleted: %v", err)
	}
}

func TestApplyRetentionPolicyBelowMinKeepsAll(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace)
	writeGen(t, nsDir, gen1, gen2)
	// maxVersions below the minimum falls back to the default, so nothing is pruned.
	err := snaputil.ApplyRetentionPolicy(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace, true, constants.DefaultInitValue)
	if err != nil {
		t.Fatalf("ApplyRetentionPolicy: %v", err)
	}
	for _, g := range []int{gen1, gen2} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+strconv.Itoa(g)+".json")); err != nil {
			t.Errorf("V%d.json should survive default retention: %v", g, err)
		}
	}
}

// Every nested map in a sanitized manifest is asserted, so a missing one has to
// fail the test rather than surface as a nil map read.
func mapAt(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	nested, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %T, want a map", key, m[key])
	}
	return nested
}

func TestSanitizeManifest(t *testing.T) {
	resources := []map[string]any{
		{
			constants.FieldKind: kindService,
			constants.MetadataField: map[string]any{
				fieldAnnotations: map[string]any{
					"deployment.kubernetes.io/revision": "3",
					"telark.io/last-modified-by":        "x",
					"keep":                              "yes",
				},
			},
			constants.SpecField: map[string]any{
				fieldInternalTrafficPolicy: "Cluster",
				"clusterIP":                "10.0.0.1",
			},
		},
		nil,
		{
			constants.FieldKind: "ConfigMap",
			constants.SpecField: map[string]any{fieldInternalTrafficPolicy: "keepme"},
		},
	}
	got := snaputil.SanitizeManifest(resources)
	if len(got) != wantSanitizedKept {
		t.Fatalf("len = %d, want %d", len(got), wantSanitizedKept)
	}
	svc := got[constants.DefaultInitValue]
	ann := mapAt(t, mapAt(t, svc, constants.MetadataField), fieldAnnotations)
	if _, ok := ann["deployment.kubernetes.io/revision"]; ok {
		t.Error("managed annotation not stripped")
	}
	if ann["keep"] != "yes" {
		t.Error("user annotation was removed")
	}
	spec := mapAt(t, svc, constants.SpecField)
	if _, ok := spec[fieldInternalTrafficPolicy]; ok {
		t.Error("cluster-managed service field not stripped")
	}
	if spec["clusterIP"] != "10.0.0.1" {
		t.Error("clusterIP wrongly removed")
	}
	// Non-Service kinds keep their spec untouched.
	if mapAt(t, got[wantOrderedItems], constants.SpecField)[fieldInternalTrafficPolicy] != "keepme" {
		t.Error("non-service spec field was stripped")
	}
}

func TestSanitizeManifestDropsEmptyAnnotations(t *testing.T) {
	resources := []map[string]any{
		{
			constants.MetadataField: map[string]any{
				fieldAnnotations: map[string]any{"telark.io/last-modified-at": "now"},
			},
		},
	}
	got := snaputil.SanitizeManifest(resources)
	meta := mapAt(t, got[constants.DefaultInitValue], constants.MetadataField)
	if _, ok := meta[fieldAnnotations]; ok {
		t.Error("annotations map should be dropped once empty")
	}
}

// The cache is process-global, so this is the one test that runs in cached
// mode and it must be the first in the package to touch it.
func waitForFirstStatsWalk() {
	deadline := time.Now().Add(statsWalkDeadline)
	for snaputil.StorageStatsWalks() == constants.DefaultInitValue && time.Now().Before(deadline) {
		time.Sleep(statsWalkPollBackoff)
	}
}

func TestStorageStatsCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(constants.SnapshotsPathEnv, dir)
	envs.InitSnapshotsPath()
	t.Setenv("SNAPSHOT_STATS_REFRESH_SEC", "60")
	envs.InitSnapshotStatsRefreshInterval()
	appDir := filepath.Join(dir, constants.SnapshotsAppsSubdir, testAppID, testNamespace)
	writeGen(t, appDir, gen1, gen2)

	// First read must not wait for the walk it kicks off.
	if infos := snaputil.BuildSnapshotInfos(); infos.UpdatedAt != constants.DefaultInitValue || infos.TotalSnapshots != constants.DefaultInitValue {
		t.Fatalf("pre-walk read = %+v, want zeros", infos)
	}
	waitForFirstStatsWalk()
	walks := snaputil.StorageStatsWalks()
	infos := snaputil.BuildSnapshotInfos()
	if walks != constants.DefaultIncrementValue || infos.TotalSnapshots != wantSnapshots ||
		infos.ConsumedSpace.Bytes != wantSnapshotBytes || infos.UpdatedAt == constants.DefaultInitValue {
		t.Fatalf("walks = %d, infos = %+v, want 1 walk, 2 snapshots, 4 bytes, updatedAt set", walks, infos)
	}

	// Reads serve the last walk even once the volume has changed.
	writeGen(t, appDir, gen3)
	infos = snaputil.BuildSnapshotInfos()
	if snaputil.StorageStatsWalks() != walks || infos.TotalSnapshots != wantSnapshots {
		t.Errorf("cached read re-walked: walks = %d, snapshots = %d", snaputil.StorageStatsWalks(), infos.TotalSnapshots)
	}

	// Only the refresher tick re-walks.
	snaputil.RefreshStorageStats()
	infos = snaputil.BuildSnapshotInfos()
	if snaputil.StorageStatsWalks() != walks+constants.DefaultIncrementValue || infos.TotalSnapshots != wantRefreshedCount {
		t.Errorf("refresh did not re-walk: walks = %d, snapshots = %d", snaputil.StorageStatsWalks(), infos.TotalSnapshots)
	}
}

func TestGetStorageInfo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(constants.SnapshotsPathEnv, dir)
	envs.InitSnapshotsPath()
	t.Setenv("SNAPSHOT_STATS_REFRESH_SEC", "0")
	envs.InitSnapshotStatsRefreshInterval()
	// No PVC backend is reachable in tests, so every field falls back to a
	// placeholder rather than panicking or returning an empty string.
	avail, total, pct := snaputil.GetStorageInfo()
	if avail == constants.EmptyString || total == constants.EmptyString || pct == constants.EmptyString {
		t.Errorf("GetStorageInfo returned empty fields: %q %q %q", avail, total, pct)
	}
}

func TestComputeSnapshotsStorageStats(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, constants.SnapshotsAppsSubdir, testAppID, testNamespace)
	writeGen(t, dir, gen1, gen2)
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), make([]byte, strayBytes), filePerm); err != nil {
		t.Fatal(err)
	}
	total, count, err := snaputil.ComputeSnapshotsStorageStats(root)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	// Two V*.json files are snapshots; the .txt adds to bytes but not the count.
	if count != wantSnapshots {
		t.Errorf("count = %d, want 2", count)
	}
	if total < strayBytes {
		t.Errorf("total = %d, want >= 10", total)
	}
}
