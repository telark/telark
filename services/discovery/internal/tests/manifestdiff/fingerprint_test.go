package manifestdiff_test

import (
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	kcoremanifest "github.com/telark/kcore/manifest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	workloadName         = "web"
	servicePort          = 80
	serviceTargetPort    = 8080
	wantNoChangesFmt     = "want no changes, got %+v"
	keyAnnotations       = "annotations"
	annotationModifiedBy = "telark.io/last-modified-by"
)

func liveDeployment(templateAnnotations map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		keyKind:      "Deployment",
		keyMetadata: map[string]any{
			keyName:           workloadName,
			"namespace":       "ns",
			"uid":             "u1",
			"resourceVersion": "42",
			"managedFields":   []any{map[string]any{"manager": "kubectl"}},
			"labels":          map[string]any{"app": workloadName},
			keyAnnotations: map[string]any{
				"deployment.kubernetes.io/revision": "1",
				annotationModifiedBy:                "someone",
			},
		},
		keySpec: map[string]any{
			keyReplicas: int64(constants.DefaultAddValue),
			"template": map[string]any{
				keyMetadata: map[string]any{"creationTimestamp": nil, keyAnnotations: templateAnnotations},
				keySpec: map[string]any{
					"containers": []any{map[string]any{
						keyName:                    workloadName,
						"image":                    "registry.k8s.io/pause:3.9",
						"terminationMessagePath":   "/dev/termination-log",
						"terminationMessagePolicy": "File",
					}},
				},
			},
		},
		"status": map[string]any{keyReplicas: int64(constants.DefaultAddValue)},
	}}
}

// What the exporter serves back: kcore's apply-clean copy minus the
// annotations and Service defaults its sanitize step drops.
func snapshotCopy(u *unstructured.Unstructured) *unstructured.Unstructured {
	out := u.DeepCopy()
	kcoremanifest.CleanManifestForApply(out.Object)
	ann := out.GetAnnotations()
	noisy := []string{"deployment.kubernetes.io/revision", "telark.io/last-modified-at", annotationModifiedBy, "telark.io/last-modified-operation"}
	for _, k := range noisy {
		delete(ann, k)
	}
	out.SetAnnotations(ann)
	if out.GetKind() == "Service" {
		for _, f := range []string{"internalTrafficPolicy", "ipFamilies", "ipFamilyPolicy", "sessionAffinity"} {
			unstructured.RemoveNestedField(out.Object, keySpec, f)
		}
	}
	return out
}

func liveService() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		keyKind:      "Service",
		keyMetadata: map[string]any{
			keyName: workloadName, "namespace": "ns", "uid": "u2", keyAnnotations: map[string]any{annotationModifiedBy: "someone"},
		},
		keySpec: map[string]any{
			"clusterIP":             "10.0.0.1",
			"clusterIPs":            []any{"10.0.0.1"},
			"internalTrafficPolicy": "Cluster",
			"ipFamilies":            []any{"IPv4"},
			"ipFamilyPolicy":        "SingleStack",
			"sessionAffinity":       "None",
			"selector":              map[string]any{"app": workloadName},
			"ports":                 []any{map[string]any{"port": int64(servicePort), "targetPort": int64(serviceTargetPort)}},
		},
	}}
}

func TestServiceFingerprintMatchesServedSnapshot(t *testing.T) {
	live := liveService()
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("live Service and its served snapshot must fingerprint identically")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != constants.DefaultInitValue {
		t.Fatalf(wantNoChangesFmt, got)
	}
	changed := liveService()
	changed.SetLabels(map[string]string{"tier": "edge"})
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: changed}}); len(got) != constants.DefaultAddValue {
		t.Fatalf("a real label change must still be reported once, got %+v", got)
	}
}

func TestFingerprintMatchesApplyCleanSnapshot(t *testing.T) {
	live := liveDeployment(nil)
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("raw live object and its apply-clean snapshot must fingerprint identically")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != constants.DefaultInitValue {
		t.Fatalf("snapshot pre-image against unchanged live must diff empty, got %d changes: %+v", len(got), got)
	}
}

func TestRestartedSnapshotStillMatchesLive(t *testing.T) {
	live := liveDeployment(map[string]any{"kubectl.kubernetes.io/restartedAt": "2026-09-18T00:00:00Z"})
	snap := snapshotCopy(live)
	if manifestdiff.Fingerprint(snap) != manifestdiff.Fingerprint(live) {
		t.Fatal("a restarted deployment must fingerprint like its snapshot, which never holds restartedAt")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != constants.DefaultInitValue {
		t.Fatalf(wantNoChangesFmt, got)
	}
}

func TestOnlyNoisyAnnotationsCompareAsNone(t *testing.T) {
	live := liveDeployment(nil)
	snap := snapshotCopy(live)
	delete(snap.Object[keyMetadata].(map[string]any), keyAnnotations)
	if manifestdiff.Fingerprint(live) != manifestdiff.Fingerprint(snap) {
		t.Fatal("an annotations map holding only noisy keys must fingerprint like no annotations at all")
	}
	if got := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: snap, New: live}}); len(got) != constants.DefaultInitValue {
		t.Fatalf(wantNoChangesFmt, got)
	}
}
