package manifestdiff_test

import (
	"testing"

	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	kcoremanifest "github.com/telark/kcore/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func liveDeployment(templateAnnotations map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]any{
			"name":            "web",
			"namespace":       "ns",
			"uid":             "u1",
			"resourceVersion": "42",
			"managedFields":   []any{map[string]any{"manager": "kubectl"}},
			"labels":          map[string]any{"app": "web"},
			"annotations": map[string]any{
				"deployment.kubernetes.io/revision": "1",
				"telark.io/last-modified-by":        "someone",
			},
		},
		"spec": map[string]any{
			"replicas": int64(1),
			"template": map[string]any{
				"metadata": map[string]any{"creationTimestamp": nil, "annotations": templateAnnotations},
				"spec": map[string]any{
					"containers": []any{map[string]any{
						"name":                     "web",
						"image":                    "registry.k8s.io/pause:3.9",
						"terminationMessagePath":   "/dev/termination-log",
						"terminationMessagePolicy": "File",
					}},
				},
			},
		},
		"status": map[string]any{"replicas": int64(1)},
	}}
}

// What the exporter serves back: kcore's apply-clean copy minus the
// annotations and Service defaults its sanitize step drops.
func snapshotCopy(u *unstructured.Unstructured) *unstructured.Unstructured {
	out := u.DeepCopy()
	kcoremanifest.CleanManifestForApply(out.Object)
	ann := out.GetAnnotations()
	for _, k := range []string{"deployment.kubernetes.io/revision", "telark.io/last-modified-at", "telark.io/last-modified-by", "telark.io/last-modified-operation"} {
		delete(ann, k)
	}
	out.SetAnnotations(ann)
	if out.GetKind() == "Service" {
		spec := out.Object["spec"].(map[string]any)
		for _, f := range []string{"internalTrafficPolicy", "ipFamilies", "ipFamilyPolicy", "sessionAffinity"} {
			delete(spec, f)
		}
	}
	return out
}

func liveService() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata":   map[string]any{"name": "web", "namespace": "ns", "uid": "u2", "annotations": map[string]any{"telark.io/last-modified-by": "someone"}},
		"spec": map[string]any{
			"clusterIP":             "10.0.0.1",
			"clusterIPs":            []any{"10.0.0.1"},
			"internalTrafficPolicy": "Cluster",
			"ipFamilies":            []any{"IPv4"},
			"ipFamilyPolicy":        "SingleStack",
			"sessionAffinity":       "None",
			"selector":              map[string]any{"app": "web"},
			"ports":                 []any{map[string]any{"port": int64(80), "targetPort": int64(8080)}},
		},
	}}
}

func TestServiceFingerprintMatchesServedSnapshot(t *testing.T) {
	live := liveService()
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("live Service and its served snapshot must fingerprint identically")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != 0 {
		t.Fatalf("want no changes, got %+v", got)
	}
	changed := liveService()
	changed.SetLabels(map[string]string{"tier": "edge"})
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: changed}}); len(got) != 1 {
		t.Fatalf("a real label change must still be reported once, got %+v", got)
	}
}

func TestFingerprintMatchesApplyCleanSnapshot(t *testing.T) {
	live := liveDeployment(nil)
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("raw live object and its apply-clean snapshot must fingerprint identically")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != 0 {
		t.Fatalf("snapshot pre-image against unchanged live must diff empty, got %d changes: %+v", len(got), got)
	}
}

func TestRestartedSnapshotStillMatchesLive(t *testing.T) {
	live := liveDeployment(map[string]any{"kubectl.kubernetes.io/restartedAt": "2026-09-18T00:00:00Z"})
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(snap) != manifestdiff.Fingerprint(live) {
		t.Fatal("a restarted deployment must fingerprint like its snapshot, which never holds restartedAt")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != 0 {
		t.Fatalf("want no changes, got %+v", got)
	}
}

func TestOnlyNoisyAnnotationsCompareAsNone(t *testing.T) {
	live := liveDeployment(nil)
	snap := snapshotCopy(live)
	delete(snap.Object["metadata"].(map[string]any), "annotations")
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("an annotations map holding only noisy keys must fingerprint like no annotations at all")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != 0 {
		t.Fatalf("want no changes, got %+v", got)
	}
}
