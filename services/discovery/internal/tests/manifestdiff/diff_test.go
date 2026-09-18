package manifestdiff_test

import (
	"strings"
	"testing"

	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func deployment(replicas int64, image string, probePeriod int64, args []any, annotations map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"kind": "Deployment",
		"metadata": map[string]any{
			"name":        "web",
			"labels":      map[string]any{"app": "web", "tier": "backend"},
			"annotations": annotations,
		},
		"spec": map[string]any{
			"replicas": replicas,
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name":  "app",
							"image": image,
							"args":  args,
							"livenessProbe": map[string]any{
								"periodSeconds": probePeriod,
							},
						},
					},
				},
			},
		},
		"status": map[string]any{"replicas": replicas},
	}}
}

func single(t *testing.T, out []changesOut, want string) changesOut {
	t.Helper()
	if len(out) != 1 {
		t.Fatalf("want exactly one change (%s), got %d: %+v", want, len(out), out)
	}
	return out[0]
}

type changesOut = struct {
	Field       string
	Description string
	ChangeType  string
	OldValue    *string
	NewValue    *string
}

func run(old, cur *unstructured.Unstructured) []changesOut {
	var out []changesOut
	for _, c := range manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: old, New: cur}}) {
		out = append(out, changesOut{c.Field, c.Description, c.ChangeType, c.OldValue, c.NewValue})
	}
	return out
}

func TestProbeChangeInsideNamedContainer(t *testing.T) {
	old := deployment(1, "img:1", 10, []any{"a"}, nil)
	cur := deployment(1, "img:1", 20, []any{"a"}, nil)
	c := single(t, run(old, cur), "probe")
	if c.Field != "Deployment/web spec.template.spec.containers[app].livenessProbe.periodSeconds" {
		t.Fatalf("field: %s", c.Field)
	}
	if c.ChangeType != changes.ChangeTypeUpdated || *c.OldValue != "10" || *c.NewValue != "20" {
		t.Fatalf("unexpected change: %+v", c)
	}
	if !changes.IsWorkloadTemplateField(c.Field) {
		t.Fatalf("template field not recognised: %s", c.Field)
	}
}

func TestCuratedPathsAndStatusAreSkipped(t *testing.T) {
	old := deployment(1, "img:1", 10, []any{"a"}, nil)
	cur := deployment(3, "img:2", 10, []any{"a"}, nil)
	cur.Object["status"] = map[string]any{"replicas": int64(99)}
	if out := run(old, cur); len(out) != 0 {
		t.Fatalf("replicas, image and status must be skipped, got %+v", out)
	}
}

func TestNoisyAnnotationIgnoredRealAnnotationReported(t *testing.T) {
	old := deployment(1, "img:1", 10, nil, map[string]any{"deployment.kubernetes.io/revision": "1"})
	cur := deployment(1, "img:1", 10, nil, map[string]any{"deployment.kubernetes.io/revision": "2", "team": "core"})
	c := single(t, run(old, cur), "annotation")
	if c.Field != "Deployment/web metadata.annotations.team" || c.ChangeType != changes.ChangeTypeAdded || *c.NewValue != "core" {
		t.Fatalf("unexpected change: %+v", c)
	}
	if changes.IsWorkloadTemplateField(c.Field) {
		t.Fatalf("metadata change must not classify as a template change")
	}
}

func TestLabelRemoved(t *testing.T) {
	old := deployment(1, "img:1", 10, nil, nil)
	cur := deployment(1, "img:1", 10, nil, nil)
	delete(cur.Object["metadata"].(map[string]any)["labels"].(map[string]any), "tier")
	c := single(t, run(old, cur), "label")
	if c.Field != "Deployment/web metadata.labels.tier" || c.ChangeType != changes.ChangeTypeRemoved || *c.OldValue != "backend" {
		t.Fatalf("unexpected change: %+v", c)
	}
}

func TestPositionalListLengthChangeReportedWhole(t *testing.T) {
	old := deployment(1, "img:1", 10, []any{"a"}, nil)
	cur := deployment(1, "img:1", 10, []any{"a", "b"}, nil)
	c := single(t, run(old, cur), "args")
	if c.Field != "Deployment/web spec.template.spec.containers[app].args" || *c.OldValue != `["a"]` || *c.NewValue != `["a","b"]` {
		t.Fatalf("unexpected change: %+v", c)
	}
}

func TestConfigMapDataKeyChange(t *testing.T) {
	mk := func(v string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"kind":     "ConfigMap",
			"metadata": map[string]any{"name": "cfg"},
			"data":     map[string]any{"LOG_LEVEL": v},
		}}
	}
	c := single(t, run(mk("info"), mk("debug")), "configmap")
	if c.Field != "ConfigMap/cfg data.LOG_LEVEL" || c.Description != "ConfigMap/cfg: data.LOG_LEVEL: info → debug" {
		t.Fatalf("unexpected change: %+v", c)
	}
}

func TestNilSideSkippedAndIdenticalQuiet(t *testing.T) {
	obj := deployment(1, "img:1", 10, nil, nil)
	if out := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: nil, New: obj}, {Old: obj, New: nil}, {Old: obj, New: obj.DeepCopy()}}); len(out) != 0 {
		t.Fatalf("expected no changes, got %+v", out)
	}
}

func TestSecretValuesAreRedacted(t *testing.T) {
	mk := func(v string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"kind":     "Secret",
			"metadata": map[string]any{"name": "creds", "labels": map[string]any{"tier": "db"}},
			"data":     map[string]any{"password": v},
		}}
	}
	out := run(mk("b2xk"), mk("bmV3"))
	c := single(t, out, "secret")
	if c.Field != "Secret/creds data.password" || *c.OldValue != "<redacted>" || *c.NewValue != "<redacted>" {
		t.Fatalf("secret value leaked: %+v", c)
	}
	if strings.Contains(c.Description, "bmV3") {
		t.Fatalf("description leaked value: %s", c.Description)
	}
}
