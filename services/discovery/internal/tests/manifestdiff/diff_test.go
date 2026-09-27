package manifestdiff_test

import (
	"strings"
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	keyKind             = "kind"
	keyMetadata         = "metadata"
	keyName             = "name"
	keySpec             = "spec"
	keyReplicas         = "replicas"
	image1              = "img:1"
	probePeriod         = 10
	probePeriodChanged  = 20
	unexpectedChangeFmt = "unexpected change: %+v"
	statusReplicas      = 99
	labelTier           = "tier"
)

func deployment(replicas int64, image string, probePeriod int64, args []any, annotations map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		keyKind: "Deployment",
		keyMetadata: map[string]any{
			keyName:       "web",
			"labels":      map[string]any{"app": "web", labelTier: "backend"},
			"annotations": annotations,
		},
		keySpec: map[string]any{
			keyReplicas: replicas,
			"template": map[string]any{
				keySpec: map[string]any{
					"containers": []any{
						map[string]any{
							keyName: "app",
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
		"status": map[string]any{keyReplicas: replicas},
	}}
}

func single(t *testing.T, out []changesOut, want string) changesOut {
	t.Helper()
	if len(out) != constants.DefaultAddValue {
		t.Fatalf("want exactly one change (%s), got %d: %+v", want, len(out), out)
	}
	return out[constants.DefaultInitValue]
}

type changesOut = struct {
	Field       string
	Description string
	ChangeType  string
	OldValue    *string
	NewValue    *string
}

func run(old, cur *unstructured.Unstructured) []changesOut {
	changed := manifestdiff.Changes([]manifestdiff.ManifestPair{{Old: old, New: cur}})
	out := make([]changesOut, constants.DefaultInitValue, len(changed))
	for _, c := range changed {
		out = append(out, changesOut{c.Field, c.Description, c.ChangeType, c.OldValue, c.NewValue})
	}
	return out
}

func TestProbeChangeInsideNamedContainer(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, []any{"a"}, nil)
	cur := deployment(constants.DefaultAddValue, image1, probePeriodChanged, []any{"a"}, nil)
	c := single(t, run(old, cur), "probe")
	if c.Field != "Deployment/web spec.template.spec.containers[app].livenessProbe.periodSeconds" {
		t.Fatalf("field: %s", c.Field)
	}
	if c.ChangeType != changes.ChangeTypeUpdated || *c.OldValue != "10" || *c.NewValue != "20" {
		t.Fatalf(unexpectedChangeFmt, c)
	}
	if !changes.IsWorkloadTemplateField(c.Field) {
		t.Fatalf("template field not recognized: %s", c.Field)
	}
}

// An app spanning namespaces can hold the same Kind/name in each: the target names the namespace.
func TestNamespacedTargetNamesTheNamespace(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, []any{"a"}, nil)
	cur := deployment(constants.DefaultAddValue, image1, probePeriodChanged, []any{"a"}, nil)
	old.SetNamespace("prod")
	cur.SetNamespace("prod")
	c := single(t, run(old, cur), "probe")
	if c.Field != "prod/Deployment/web spec.template.spec.containers[app].livenessProbe.periodSeconds" {
		t.Fatalf("field: %s", c.Field)
	}
	if !strings.HasPrefix(c.Description, "prod/Deployment/web: ") || !changes.IsWorkloadTemplateField(c.Field) {
		t.Fatalf(unexpectedChangeFmt, c)
	}
}

func TestCuratedPathsAndStatusAreSkipped(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, []any{"a"}, nil)
	cur := deployment(constants.ThreeValue, "img:2", probePeriod, []any{"a"}, nil)
	cur.Object["status"] = map[string]any{keyReplicas: int64(statusReplicas)}
	if out := run(old, cur); len(out) != constants.DefaultInitValue {
		t.Fatalf("replicas, image and status must be skipped, got %+v", out)
	}
}

func TestNoisyAnnotationIgnoredRealAnnotationReported(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, nil, map[string]any{"deployment.kubernetes.io/revision": "1"})
	cur := deployment(constants.DefaultAddValue, image1, probePeriod, nil, map[string]any{"deployment.kubernetes.io/revision": "2", "team": "core"})
	c := single(t, run(old, cur), "annotation")
	if c.Field != "Deployment/web metadata.annotations.team" || c.ChangeType != changes.ChangeTypeAdded || *c.NewValue != "core" {
		t.Fatalf(unexpectedChangeFmt, c)
	}
	if changes.IsWorkloadTemplateField(c.Field) {
		t.Fatal("metadata change must not classify as a template change")
	}
}

func TestLabelRemoved(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, nil, nil)
	cur := deployment(constants.DefaultAddValue, image1, probePeriod, nil, nil)
	delete(cur.Object[keyMetadata].(map[string]any)["labels"].(map[string]any), labelTier)
	c := single(t, run(old, cur), "label")
	if c.Field != "Deployment/web metadata.labels.tier" || c.ChangeType != changes.ChangeTypeRemoved || *c.OldValue != "backend" {
		t.Fatalf(unexpectedChangeFmt, c)
	}
}

func TestPositionalListLengthChangeReportedWhole(t *testing.T) {
	old := deployment(constants.DefaultAddValue, image1, probePeriod, []any{"a"}, nil)
	cur := deployment(constants.DefaultAddValue, image1, probePeriod, []any{"a", "b"}, nil)
	c := single(t, run(old, cur), "args")
	if c.Field != "Deployment/web spec.template.spec.containers[app].args" || *c.OldValue != `["a"]` || *c.NewValue != `["a","b"]` {
		t.Fatalf(unexpectedChangeFmt, c)
	}
}

func TestConfigMapDataKeyChange(t *testing.T) {
	mk := func(v string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			keyKind:     "ConfigMap",
			keyMetadata: map[string]any{keyName: "cfg"},
			"data":      map[string]any{"LOG_LEVEL": v},
		}}
	}
	c := single(t, run(mk("info"), mk("debug")), "configmap")
	if c.Field != "ConfigMap/cfg data.LOG_LEVEL" || c.Description != "ConfigMap/cfg: data.LOG_LEVEL: info → debug" {
		t.Fatalf(unexpectedChangeFmt, c)
	}
}

func TestNilSideSkippedAndIdenticalQuiet(t *testing.T) {
	obj := deployment(constants.DefaultAddValue, image1, probePeriod, nil, nil)
	pairs := []manifestdiff.ManifestPair{{Old: nil, New: obj}, {Old: obj, New: nil}, {Old: obj, New: obj.DeepCopy()}}
	if out := manifestdiff.Changes(pairs); len(out) != constants.DefaultInitValue {
		t.Fatalf("expected no changes, got %+v", out)
	}
}

func TestSecretValuesAreRedacted(t *testing.T) {
	mk := func(v string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			keyKind:     "Secret",
			keyMetadata: map[string]any{keyName: "creds", "labels": map[string]any{labelTier: "db"}},
			"data":      map[string]any{"password": v},
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
