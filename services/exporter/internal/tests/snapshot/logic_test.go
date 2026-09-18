package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/telark/exporter/internal/managers/envs"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes uint64
		want  string
	}{
		{"bytes", 512, "512 B"},
		{"kilobytes", 2048, "2.00 KB"},
		{"megabytes", 3 * 1024 * 1024, "3.00 MB"},
		{"gigabytes", 1024 * 1024 * 1024, "1.00 GB"},
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
	if err := os.WriteFile(path, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := snaputil.FormattedFileSize(path, "id"); got != "2.00 KB" {
		t.Errorf("FormattedFileSize existing = %q, want 2.00 KB", got)
	}
	if got := snaputil.FormattedFileSize(filepath.Join(dir, "missing"), "id"); got != "unknown" {
		t.Errorf("FormattedFileSize missing = %q, want unknown", got)
	}
}

func TestBuildKubernetesItems(t *testing.T) {
	valid := map[string]any{
		"manifest": map[string]any{
			"resources": []any{
				map[string]any{"manifest": map[string]any{"kind": "Deployment"}},
				map[string]any{"manifest": map[string]any{"kind": "Service"}},
			},
		},
	}
	items, ok := snaputil.BuildKubernetesItems(valid)
	if !ok || len(items) != 2 {
		t.Fatalf("BuildKubernetesItems ok=%v len=%d, want true/2", ok, len(items))
	}
	// Service applies before Deployment, so the sort must reorder them.
	if items[0]["kind"] != "Service" || items[1]["kind"] != "Deployment" {
		t.Errorf("items not ordered by apply weight: %v, %v", items[0]["kind"], items[1]["kind"])
	}

	bad := []struct {
		name string
		in   map[string]any
	}{
		{"no manifest key", map[string]any{}},
		{"manifest not map", map[string]any{"manifest": "x"}},
		{"resources not slice", map[string]any{"manifest": map[string]any{"resources": "x"}}},
		{"resource not map", map[string]any{"manifest": map[string]any{"resources": []any{"x"}}}},
		{"resource without inner manifest", map[string]any{"manifest": map[string]any{"resources": []any{map[string]any{}}}}},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := snaputil.BuildKubernetesItems(tt.in); ok {
				t.Errorf("BuildKubernetesItems(%v) ok=true, want false", tt.in)
			}
		})
	}
}

func TestBuildKubernetesListPayload(t *testing.T) {
	valid := map[string]any{
		"manifest": map[string]any{
			"resources": []any{map[string]any{"manifest": map[string]any{"kind": "Service"}}},
		},
	}
	payload, ok := snaputil.BuildKubernetesListPayload(valid)
	if !ok {
		t.Fatal("BuildKubernetesListPayload ok=false, want true")
	}
	if payload["apiVersion"] != "v1" || payload["kind"] != "List" {
		t.Errorf("payload envelope wrong: %v", payload)
	}
	if _, ok := payload["items"].([]map[string]any); !ok {
		t.Errorf("items missing from payload: %v", payload)
	}
	if _, ok := snaputil.BuildKubernetesListPayload(map[string]any{}); ok {
		t.Error("empty snapshot produced a payload")
	}
}

func TestIsWithinBase(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		base       string
		wantWithin bool
	}{
		{"child path", "/base/sub/x.json", "/base", true},
		{"equal path", "/base", "/base", false},
		{"sibling prefix no separator", "/basex", "/base", false},
		{"outside", "/other/x", "/base", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snaputil.IsWithinBase(tt.target, tt.base); got != tt.wantWithin {
				t.Errorf("IsWithinBase(%q,%q) = %v, want %v", tt.target, tt.base, got, tt.wantWithin)
			}
		})
	}
}

func TestBuildSnapshotDirAndPath(t *testing.T) {
	if got := snaputil.BuildSnapshotDir("/s", "apps", "id", "ns", true); got != "/s/apps/id/ns" {
		t.Errorf("namespaced dir = %q", got)
	}
	if got := snaputil.BuildSnapshotDir("/s", "apps", "id", "ns", false); got != "/s/apps/id" {
		t.Errorf("cluster-scoped dir = %q", got)
	}
	if got := snaputil.BuildSnapshotPath("/s", "apps", "id", "ns", 3, true); got != "/s/apps/id/ns/V3.json" {
		t.Errorf("namespaced path = %q", got)
	}
	if got := snaputil.BuildSnapshotPath("/s", "apps", "id", "ns", 3, false); got != "/s/apps/id/V3.json" {
		t.Errorf("cluster-scoped path = %q", got)
	}
}

func TestValidateSnapshotIdentity(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		scope   string
		wantErr bool
	}{
		{"valid", "app-1", "apps", false},
		{"empty id", "", "apps", true},
		{"empty scope", "app-1", "", true},
		{"traversal id", "..", "apps", true},
		{"path in id", "a/b", "apps", true},
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

func TestParseCreatePayload(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"id": "app-1", "scope": "apps", "manifest": map[string]any{"x": 1}, "generation": float64(2), "namespace": " ns "}
	}
	t.Run("valid", func(t *testing.T) {
		got, err := snaputil.ParseCreatePayload(base())
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if got.ID != "app-1" || got.Scope != "apps" || got.Namespace != "ns" || got.Generation != 2 {
			t.Errorf("payload = %+v", got)
		}
	})

	mutate := map[string]func(m map[string]any){
		"missing id":       func(m map[string]any) { delete(m, "id") },
		"id not string":    func(m map[string]any) { m["id"] = 1 },
		"empty id":         func(m map[string]any) { m["id"] = "" },
		"missing scope":    func(m map[string]any) { delete(m, "scope") },
		"scope not string": func(m map[string]any) { m["scope"] = 1 },
		"empty scope":      func(m map[string]any) { m["scope"] = "" },
		"missing manifest": func(m map[string]any) { delete(m, "manifest") },
		"nil manifest":     func(m map[string]any) { m["manifest"] = nil },
		"missing gen":      func(m map[string]any) { delete(m, "generation") },
		"zero gen":         func(m map[string]any) { m["generation"] = float64(0) },
		"fractional gen":   func(m map[string]any) { m["generation"] = float64(2.5) },
		"bad string gen":   func(m map[string]any) { m["generation"] = "abc" },
		"unsupported gen":  func(m map[string]any) { m["generation"] = []int{1} },
	}
	for name, mut := range mutate {
		t.Run(name, func(t *testing.T) {
			m := base()
			mut(m)
			if _, err := snaputil.ParseCreatePayload(m); err == nil {
				t.Errorf("%s: expected error", name)
			}
		})
	}

	t.Run("gen as int and int64 and string", func(t *testing.T) {
		for _, v := range []any{int(3), int64(4), "5"} {
			m := base()
			m["generation"] = v
			if _, err := snaputil.ParseCreatePayload(m); err != nil {
				t.Errorf("generation %v(%T) rejected: %v", v, v, err)
			}
		}
	})

	t.Run("namespace not string defaults empty", func(t *testing.T) {
		m := base()
		m["namespace"] = 42
		got, err := snaputil.ParseCreatePayload(m)
		if err != nil || got.Namespace != "" {
			t.Errorf("non-string namespace: got %q err %v", got.Namespace, err)
		}
	})
}

func TestParseCreateSnapshotRequest(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"id": "app-1", "scope": "apps", "namespace": "ns", "generation": float64(1), "manifest": map[string]any{"x": 1}}
	}
	t.Run("valid", func(t *testing.T) {
		got, err := snaputil.ParseCreateSnapshotRequest(base())
		if err != nil || got.ID != "app-1" || got.Scope != "apps" || got.Namespace != "ns" {
			t.Fatalf("ParseCreateSnapshotRequest = %+v, err %v", got, err)
		}
	})
	mutate := map[string]func(m map[string]any){
		"missing id":            func(m map[string]any) { delete(m, "id") },
		"missing scope":         func(m map[string]any) { delete(m, "scope") },
		"unregistered scope":    func(m map[string]any) { m["scope"] = "nope" },
		"namespaced without ns": func(m map[string]any) { delete(m, "namespace") },
		"missing generation":    func(m map[string]any) { delete(m, "generation") },
		"missing manifest":      func(m map[string]any) { delete(m, "manifest") },
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
	if got := snaputil.RegisteredScopesText(envs.GetSnapshotScopes()); got != "apps" {
		t.Errorf("RegisteredScopesText = %q, want apps", got)
	}
}

func TestSnapshotNotFoundMessage(t *testing.T) {
	if snaputil.SnapshotNotFoundMessage("id", "apps", "ns", "") == "" {
		t.Error("empty message for latest")
	}
	if snaputil.SnapshotNotFoundMessage("id", "apps", "ns", "3") == "" {
		t.Error("empty message for explicit generation")
	}
}

func TestContentDispositionFilename(t *testing.T) {
	got := snaputil.ContentDispositionFilename("app-1", 3)
	want := `attachment; filename="app-1-G3-rollback.json"`
	if got != want {
		t.Errorf("ContentDispositionFilename = %q, want %q", got, want)
	}
}

func TestLoadSnapshotData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()

	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"kind":"Service"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := snaputil.LoadSnapshotData(good)
	if err != nil || data["kind"] != "Service" {
		t.Fatalf("LoadSnapshotData good = %v, err %v", data, err)
	}

	if _, err := snaputil.LoadSnapshotData("/etc/hosts"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("outside base err = %v, want ErrNotExist", err)
	}
	if _, err := snaputil.LoadSnapshotData(filepath.Join(dir, "missing.json")); err == nil {
		t.Error("missing file in base: expected error")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := snaputil.LoadSnapshotData(bad); err == nil {
		t.Error("invalid json: expected error")
	}
}

func writeGen(t *testing.T, dir string, gens ...int) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, g := range gens {
		p := filepath.Join(dir, "V"+itoa(g)+".json")
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestResolveSnapshotPathNamespaced(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, "apps", "app-1", "ns")
	writeGen(t, nsDir, 1, 3, 2)

	t.Run("latest", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "ns", "", true)
		if err != nil || target.Generation != 3 {
			t.Fatalf("latest = %+v, err %v", target, err)
		}
	})
	t.Run("explicit found", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "ns", "2", true)
		if err != nil || target.Generation != 2 {
			t.Fatalf("explicit = %+v, err %v", target, err)
		}
	})
	t.Run("explicit missing", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "ns", "9", true); err == nil {
			t.Error("missing generation: expected error")
		}
	})
	t.Run("invalid generation", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "ns", "abc", true); err == nil {
			t.Error("invalid generation: expected error")
		}
	})
}

func TestResolveSnapshotPathAcrossNamespaces(t *testing.T) {
	root := t.TempDir()
	writeGen(t, filepath.Join(root, "apps", "app-1", "ns1"), 1)
	writeGen(t, filepath.Join(root, "apps", "app-1", "ns2"), 4)

	t.Run("latest picks highest across namespaces", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "", "latest", true)
		if err != nil || target.Generation != 4 || target.Namespace != "ns2" {
			t.Fatalf("across latest = %+v, err %v", target, err)
		}
	})
	t.Run("explicit across namespaces", func(t *testing.T) {
		target, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "", "1", true)
		if err != nil || target.Generation != 1 || target.Namespace != "ns1" {
			t.Fatalf("across explicit = %+v, err %v", target, err)
		}
	})
	t.Run("base dir missing", func(t *testing.T) {
		if _, err := snaputil.ResolveSnapshotPath(root, "apps", "absent", "", "latest", true); err == nil {
			t.Error("missing base: expected error")
		}
	})
}

func TestResolveSnapshotPathClusterScoped(t *testing.T) {
	root := t.TempDir()
	writeGen(t, filepath.Join(root, "apps", "app-1"), 5, 7)
	target, err := snaputil.ResolveSnapshotPath(root, "apps", "app-1", "", "", false)
	if err != nil || target.Generation != 7 {
		t.Fatalf("cluster-scoped latest = %+v, err %v", target, err)
	}
}

func TestApplyRetentionPolicy(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, "apps", "app-1", "ns")
	writeGen(t, nsDir, 1, 2, 3, 4, 5, 6)
	// A stray non-version file must never be counted or deleted.
	stray := filepath.Join(nsDir, "notes.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := snaputil.ApplyRetentionPolicy(root, "apps", "app-1", "ns", true, 3); err != nil {
		t.Fatalf("ApplyRetentionPolicy: %v", err)
	}
	for _, g := range []int{1, 2, 3} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+itoa(g)+".json")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("V%d.json should have been pruned", g)
		}
	}
	for _, g := range []int{4, 5, 6} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+itoa(g)+".json")); err != nil {
			t.Errorf("V%d.json should have been kept: %v", g, err)
		}
	}
	if _, err := os.Stat(stray); err != nil {
		t.Errorf("stray file was deleted: %v", err)
	}
}

func TestApplyRetentionPolicyBelowMinKeepsAll(t *testing.T) {
	root := t.TempDir()
	nsDir := filepath.Join(root, "apps", "app-1", "ns")
	writeGen(t, nsDir, 1, 2)
	// maxVersions below the minimum falls back to the default, so nothing is pruned.
	if err := snaputil.ApplyRetentionPolicy(root, "apps", "app-1", "ns", true, 0); err != nil {
		t.Fatalf("ApplyRetentionPolicy: %v", err)
	}
	for _, g := range []int{1, 2} {
		if _, err := os.Stat(filepath.Join(nsDir, "V"+itoa(g)+".json")); err != nil {
			t.Errorf("V%d.json should survive default retention: %v", g, err)
		}
	}
}

func TestSanitizeManifest(t *testing.T) {
	resources := []map[string]any{
		{
			"kind": "Service",
			"metadata": map[string]any{
				"annotations": map[string]any{
					"deployment.kubernetes.io/revision": "3",
					"telark.io/last-modified-by":        "x",
					"keep":                              "yes",
				},
			},
			"spec": map[string]any{
				"internalTrafficPolicy": "Cluster",
				"clusterIP":             "10.0.0.1",
			},
		},
		nil,
		{
			"kind": "ConfigMap",
			"spec": map[string]any{"internalTrafficPolicy": "keepme"},
		},
	}
	got := snaputil.SanitizeManifest(resources)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	svc := got[0]
	ann := svc["metadata"].(map[string]any)["annotations"].(map[string]any)
	if _, ok := ann["deployment.kubernetes.io/revision"]; ok {
		t.Error("managed annotation not stripped")
	}
	if ann["keep"] != "yes" {
		t.Error("user annotation was removed")
	}
	spec := svc["spec"].(map[string]any)
	if _, ok := spec["internalTrafficPolicy"]; ok {
		t.Error("cluster-managed service field not stripped")
	}
	if spec["clusterIP"] != "10.0.0.1" {
		t.Error("clusterIP wrongly removed")
	}
	// Non-Service kinds keep their spec untouched.
	if got[2]["spec"].(map[string]any)["internalTrafficPolicy"] != "keepme" {
		t.Error("non-service spec field was stripped")
	}
}

func TestSanitizeManifestDropsEmptyAnnotations(t *testing.T) {
	resources := []map[string]any{
		{
			"metadata": map[string]any{
				"annotations": map[string]any{"telark.io/last-modified-at": "now"},
			},
		},
	}
	got := snaputil.SanitizeManifest(resources)
	meta := got[0]["metadata"].(map[string]any)
	if _, ok := meta["annotations"]; ok {
		t.Error("annotations map should be dropped once empty")
	}
}

// The cache is process-global, so this is the one test that runs in cached
// mode and it must be the first in the package to touch it.
func TestStorageStatsCache(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()
	t.Setenv("SNAPSHOT_STATS_REFRESH_SEC", "60")
	envs.InitSnapshotStatsRefreshInterval()
	appDir := filepath.Join(dir, "apps", "app-1", "ns")
	writeGen(t, appDir, 1, 2)

	// First read must not wait for the walk it kicks off.
	if infos := snaputil.BuildSnapshotInfos(); infos.UpdatedAt != 0 || infos.TotalSnapshots != 0 {
		t.Fatalf("pre-walk read = %+v, want zeros", infos)
	}
	deadline := time.Now().Add(5 * time.Second)
	for snaputil.StorageStatsWalks() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	walks := snaputil.StorageStatsWalks()
	infos := snaputil.BuildSnapshotInfos()
	if walks != 1 || infos.TotalSnapshots != 2 || infos.ConsumedSpace.Bytes != 4 || infos.UpdatedAt == 0 {
		t.Fatalf("walks = %d, infos = %+v, want 1 walk, 2 snapshots, 4 bytes, updatedAt set", walks, infos)
	}

	// Reads serve the last walk even once the volume has changed.
	writeGen(t, appDir, 3)
	infos = snaputil.BuildSnapshotInfos()
	if snaputil.StorageStatsWalks() != walks || infos.TotalSnapshots != 2 {
		t.Errorf("cached read re-walked: walks = %d, snapshots = %d", snaputil.StorageStatsWalks(), infos.TotalSnapshots)
	}

	// Only the refresher tick re-walks.
	snaputil.RefreshStorageStats()
	infos = snaputil.BuildSnapshotInfos()
	if snaputil.StorageStatsWalks() != walks+1 || infos.TotalSnapshots != 3 {
		t.Errorf("refresh did not re-walk: walks = %d, snapshots = %d", snaputil.StorageStatsWalks(), infos.TotalSnapshots)
	}
}

func TestGetStorageInfo(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SNAPSHOTS_PATH", dir)
	envs.InitSnapshotsPath()
	t.Setenv("SNAPSHOT_STATS_REFRESH_SEC", "0")
	envs.InitSnapshotStatsRefreshInterval()
	// No PVC backend is reachable in tests, so every field falls back to a
	// placeholder rather than panicking or returning an empty string.
	avail, total, pct := snaputil.GetStorageInfo()
	if avail == "" || total == "" || pct == "" {
		t.Errorf("GetStorageInfo returned empty fields: %q %q %q", avail, total, pct)
	}
}

func TestComputeSnapshotsStorageStats(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "apps", "app-1", "ns")
	writeGen(t, dir, 1, 2)
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), make([]byte, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	total, count, err := snaputil.ComputeSnapshotsStorageStats(root)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	// Two V*.json files are snapshots; the .txt adds to bytes but not the count.
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
	if total < 10 {
		t.Errorf("total = %d, want >= 10", total)
	}
}
