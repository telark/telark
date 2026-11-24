package install

import (
	"net/http"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	listCategories = "GET categories.telark.io/"
	listPlans      = "GET protectionplans.telark.io/"
	openedAt       = "2026-01-01T00:00:00Z"
)

func TestUpgradeStoredObjects(t *testing.T) {
	valid := category("releases", map[string]any{"title": "Releases", "parent": "root", "color": "green", "tags": []any{"a", "b"}})
	defaulted := category("defaulted", map[string]any{"title": "Defaulted"})
	untitled := category("untitled", map[string]any{"color": "red"})
	own := category("own-parent", map[string]any{"title": "loop", "parent": "loop"})
	repeated := category("repeated", map[string]any{"title": "Repeated", "tags": []any{"a", "a"}})
	mistyped := category("mistyped", map[string]any{"title": 5})
	doubled := testutil.Resource("telark.io/v1alpha1", "AccessRole", namespace, "doubled", map[string]any{"spec": map[string]any{
		"rules": []any{map[string]any{"resource": "plans"}, map[string]any{"resource": "plans"}},
	}})
	legacy := category("legacy", map[string]any{"title": "Legacy", "icon": "star"})
	expect(t, map[string]checkCase{
		"objects that fit, one through a default": {
			objects: []runtime.Object{valid, defaulted}, line: "✔ Every stored object (2) fits the 0.2.1 schemas\n",
		},
		"objects the schema, its rules and its list types refuse": {
			objects: []runtime.Object{valid, untitled, own, repeated, mistyped, legacy, doubled}, code: failed,
			line: "× Stored objects that don't fit the 0.2.1 schemas (5): " +
				`accessroles/doubled: spec.rules[1]: Duplicate value: {"resource":"plans"}, ` +
				"categories/untitled: spec.title: Required value, " +
				"categories/own-parent: spec: Invalid value: a category can't be its own parent\n",
		},
		"a field the schema drops": {
			objects: []runtime.Object{valid, legacy},
			line:    "‼ The 0.2.1 schemas drop fields from stored objects (1): categories/legacy: spec.icon\n",
		},
		"a chart without its CRDs": {
			objects: []runtime.Object{untitled}, args: []string{setFlag, "crds.enabled=false"},
			line: "‼ The chart installs no CRDs (crds.enabled=false), so the stored objects aren't checked\n",
		},
		"a CRD the server doesn't have yet": {
			refused: map[string]int32{listCategories: http.StatusNotFound}, line: "✔ Every stored object (0) fits the 0.2.1 schemas\n",
		},
		"a chart object with fields a CRD types differently": {
			args: []string{setFlag, "sample.versions=latest"}, line: "✔ Every stored object (0) fits the 0.2.1 schemas\n",
		},
		"a CRD that doesn't read as one": {
			args: []string{setFlag, "crds.conversion=none"}, code: failed,
			line: "× Couldn't read the stored objects: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string",
		},
		"a CRD with a reference that isn't one": {
			args: []string{setFlag, "crds.reference=%"}, code: failed, line: "× Couldn't read the stored objects: parse ",
		},
		"a CRD whose schema isn't structural": {
			args: []string{setFlag, "crds.reference=#/definitions/owner"}, code: failed,
			line: "× Couldn't read the stored objects: OpenAPIV3Schema '$ref' is not supported\n",
		},
		"unreadable objects": {
			refused: map[string]int32{listCategories: http.StatusInternalServerError}, code: failed,
			line: "× Couldn't read the stored objects: ",
		},
		"a list answer without a body": {
			refused: map[string]int32{listCategories: http.StatusNoContent}, code: failed,
			line: "× Couldn't read the stored objects: unexpected end of JSON input\n",
		},
	})
}

func TestUpgradeWarnings(t *testing.T) {
	request := map[string]any{"id": "r1", "kind": "recover", "state": "approved", "openedAt": openedAt}
	expect(t, map[string]checkCase{
		"an open recovery request": {
			objects: []runtime.Object{testutil.TelarkConfig(map[string]any{
				"email": adminEmail, "pinnedAt": openedAt, "requests": []any{request},
			})},
			line: "‼ Request r1 (recover) is open and can complete at 2026-01-03T00:00:00Z\n",
		},
		"an unreadable recovery request": {
			refused: map[string]int32{"GET telarkconfigs.telark.io/default": http.StatusInternalServerError},
			line:    "‼ Couldn't read TelarkConfig default: ",
		},
		"enforcing plans": {
			objects: []runtime.Object{plan("freeze", "enforce", "active"), plan("audit", "audit", "active"), plan("done", "enforce", "terminated")},
			line: "‼ Protection plans enforce now (1): Kyverno restarts during the upgrade, and its failure policy " +
				"decides what happens to changes meanwhile\n",
		},
		"unreadable plans": {
			refused: map[string]int32{listPlans: http.StatusInternalServerError}, line: "‼ Couldn't read the protection plans: ",
		},
		"a plan that isn't one": {
			objects: []runtime.Object{plan("odd", 5, "active")}, line: "‼ Couldn't read the protection plans: ",
		},
	})
}

func plan(name string, mode any, phase string) runtime.Object {
	return testutil.Resource("telark.io/v1alpha1", "ProtectionPlan", namespace, name,
		map[string]any{"spec": map[string]any{"mode": mode}, "status": map[string]any{"phase": phase}})
}
