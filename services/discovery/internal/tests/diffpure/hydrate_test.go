package diffpure

import (
	"context"
	"errors"
	"testing"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func containsStr(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func hydrateDeployment() unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"kind":     "Deployment",
		"metadata": map[string]any{"name": "web", "namespace": "prod"},
		"spec": map[string]any{
			"replicas": int64(3),
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"image": "nginx:1.25",
							"ports": []any{map[string]any{"containerPort": int64(8080)}},
							"env": []any{
								map[string]any{"name": "LOG_LEVEL", "value": "info"},
								map[string]any{"name": "DB", "valueFrom": map[string]any{
									"configMapKeyRef": map[string]any{"name": "app-config"},
								}},
								map[string]any{"name": "PW", "valueFrom": map[string]any{
									"secretKeyRef": map[string]any{"name": "app-secret"},
								}},
							},
							"resources": map[string]any{
								"requests": map[string]any{"cpu": "100m", "memory": "128Mi"},
								"limits":   map[string]any{"cpu": "500m", "memory": "256Mi"},
							},
						},
					},
					"volumes": []any{
						map[string]any{"configMap": map[string]any{"name": "vol-config"}},
						map[string]any{"secret": map[string]any{"secretName": "vol-secret"}},
					},
				},
			},
		},
	}}
}

func hydrateService() unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"kind":     "Service",
		"metadata": map[string]any{"name": "web-svc", "namespace": "prod"},
		"spec": map[string]any{
			"type":  "ClusterIP",
			"ports": []any{map[string]any{"protocol": "TCP", "port": int64(80), "targetPort": int64(8080)}},
		},
	}}
}

func hydrateIngress() unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"kind":     "Ingress",
		"metadata": map[string]any{"name": "web-ing", "namespace": "prod"},
		"spec": map[string]any{
			"rules": []any{map[string]any{
				"host": "example.com",
				"http": map[string]any{"paths": []any{map[string]any{
					"path":    "/",
					"backend": map[string]any{"service": map[string]any{"name": "web-svc"}},
				}}},
			}},
		},
	}}
}

func manifestGetter(objs []unstructured.Unstructured) func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
	return func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		return objs, nil
	}
}

// A first-generation snapshot hydrates every derived field on an otherwise-empty
// application and seeds a workload baseline from the pre-change manifest.
func TestHydrateApplicationFromV1SnapshotsFull(t *testing.T) {
	objs := []unstructured.Unstructured{hydrateDeployment(), hydrateService(), hydrateIngress()}
	snaps := []application.ApplicationSnapshot{{Generation: 1, ID: "snap1", Namespace: "prod"}}
	app := &application.Application{
		Resources: []application.Resource{{Namespace: "prod", Kind: "Deployment", Name: "web"}},
	}

	diff.HydrateApplicationFromV1Snapshots(context.Background(), app, snaps, manifestGetter(objs))

	if !containsStr(app.Images, "nginx:1.25") {
		t.Fatalf("images = %v, want nginx:1.25", app.Images)
	}
	if len(app.Ports) != 1 || app.Ports[0] != 8080 {
		t.Fatalf("ports = %v, want [8080]", app.Ports)
	}
	for _, k := range []string{"LOG_LEVEL", "DB", "PW"} {
		if !containsStr(app.EnvVarKeys, k) {
			t.Fatalf("env keys %v missing %s", app.EnvVarKeys, k)
		}
	}
	if !containsStr(app.ConfigMapRefs, "app-config") || !containsStr(app.ConfigMapRefs, "vol-config") {
		t.Fatalf("configmap refs = %v", app.ConfigMapRefs)
	}
	if !containsStr(app.SecretRefs, "app-secret") || !containsStr(app.SecretRefs, "vol-secret") {
		t.Fatalf("secret refs = %v", app.SecretRefs)
	}
	testutil.Equal(t, "service mappings", len(app.ServiceMappings), 1)
	testutil.Equal(t, "ingress rules", len(app.IngressRules), 1)
	if len(app.Metrics.Workloads) != 1 || app.Metrics.Workloads[0].Baseline.Replicas != 3 {
		t.Fatalf("seeded workloads = %+v, want one with replicas 3", app.Metrics.Workloads)
	}
}

// Hydration never overwrites fields the application already has, and a matching
// pre-existing workload receives the baseline in place.
func TestHydrateApplicationFromV1SnapshotsPreserves(t *testing.T) {
	objs := []unstructured.Unstructured{hydrateDeployment()}
	snaps := []application.ApplicationSnapshot{{Generation: 1, ID: "snap1", Namespace: "prod"}}
	app := &application.Application{
		Images: []string{"preexisting"},
		Metrics: application.ApplicationMetrics{Workloads: []application.WorkloadUsage{
			{Namespace: "prod", ResourceKind: "Deployment", ResourceName: "web"},
		}},
	}

	diff.HydrateApplicationFromV1Snapshots(context.Background(), app, snaps, manifestGetter(objs))

	if len(app.Images) != 1 || app.Images[0] != "preexisting" {
		t.Fatalf("images overwritten = %v", app.Images)
	}
	testutil.Equal(t, "baseline replicas", app.Metrics.Workloads[0].Baseline.Replicas, int32(3))
}

// Guard paths short-circuit without panicking or mutating the application.
func TestHydrateApplicationFromV1SnapshotsGuards(t *testing.T) {
	ctx := context.Background()
	snaps := []application.ApplicationSnapshot{{Generation: 1, ID: "snap1", Namespace: "prod"}}
	getter := manifestGetter([]unstructured.Unstructured{hydrateDeployment()})

	// nil app and nil getter are both no-ops.
	diff.HydrateApplicationFromV1Snapshots(ctx, nil, snaps, getter)
	app := &application.Application{}
	diff.HydrateApplicationFromV1Snapshots(ctx, app, snaps, nil)
	testutil.Equal(t, "nil getter no-op", len(app.Images), 0)

	// A non-first generation snapshot is skipped.
	diff.HydrateApplicationFromV1Snapshots(ctx, app, []application.ApplicationSnapshot{{Generation: 2, ID: "s", Namespace: "prod"}}, getter)
	testutil.Equal(t, "gen 2 skipped", len(app.Images), 0)

	// A getter error is swallowed and skips the snapshot.
	errGetter := func(context.Context, string, string, string, int) ([]unstructured.Unstructured, error) {
		return nil, errors.New("boom")
	}
	diff.HydrateApplicationFromV1Snapshots(ctx, app, snaps, errGetter)
	testutil.Equal(t, "getter error skipped", len(app.Images), 0)
}
