package crds

import (
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/telark/kcore/crds/view"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func terminatingWidget(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	obj := widget(map[string]any{TestSizeKey: TestSizeValue}, map[string]any{TestPhaseKey: TestPhaseActive})
	stamp, err := time.Parse(time.RFC3339, TestDeletionStamp)
	if err != nil {
		t.Fatal(err)
	}
	ts := k8smetav1.NewTime(stamp)
	obj.SetDeletionTimestamp(&ts)
	obj.SetResourceVersion(TestResourceVersion)
	return obj
}

func TestToView(t *testing.T) {
	status := map[string]any{TestPhaseKey: TestPhaseActive, TestClusterKey: TestSizeValue}
	cases := []struct {
		name         string
		obj          *unstructured.Unstructured
		statusFields map[string]string
		want         map[string]any
	}{
		{"spec only plus id", widget(map[string]any{TestSizeKey: TestSizeValue}, status), nil,
			map[string]any{TestSizeKey: TestSizeValue}},
		{"renamed status key", widget(map[string]any{TestSizeKey: TestSizeValue}, status),
			map[string]string{TestPhaseView: TestPhaseKey},
			map[string]any{TestSizeKey: TestSizeValue, TestPhaseView: TestPhaseActive}},
		{"whole status object", widget(map[string]any{}, status),
			map[string]string{TestWholeView: TestWholeStatusKey},
			map[string]any{TestWholeView: status}},
		{"missing status key is not projected", widget(map[string]any{}, map[string]any{}),
			map[string]string{TestPhaseKey: TestPhaseKey}, map[string]any{}},
		{"stored spec id is overridden by metadata.name", widget(map[string]any{TestIDKey: TestSizeValue}, nil), nil,
			map[string]any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := view.ToView(tc.obj, widgetMetadata(tc.statusFields))
			want := maps.Clone(tc.want)
			want[TestIDKey] = TestName
			want[TestMetadataKey] = map[string]any{TestNameKey: TestName, TestNamespaceKey: TestNamespace}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf(ExpectedField, tc.name, "view", got, want)
			}
		})
	}
}

func TestToViewTerminating(t *testing.T) {
	got := view.ToView(terminatingWidget(t), widgetMetadata(map[string]string{TestPhaseKey: TestPhaseKey}))
	if got[TestDeletionKey] != TestDeletionStamp {
		t.Fatalf(ExpectedField, t.Name(), TestDeletionKey, got[TestDeletionKey], TestDeletionStamp)
	}
	meta, ok := got[TestMetadataKey].(map[string]any)
	if !ok || meta[TestRVKey] != TestResourceVersion {
		t.Fatalf(ExpectedField, t.Name(), TestMetadataKey, meta, TestResourceVersion)
	}
	if got[TestPhaseKey] != TestPhaseActive {
		t.Fatalf(ExpectedField, t.Name(), TestPhaseKey, got[TestPhaseKey], TestPhaseActive)
	}
}

func TestToViewDoesNotMutateSpec(t *testing.T) {
	obj := terminatingWidget(t)
	view.ToView(obj, widgetMetadata(nil))
	spec, ok := obj.Object[TestSpecKey].(map[string]any)
	if !ok {
		t.Fatalf(ExpectedMissing, t.Name(), TestSpecKey)
	}
	for _, key := range []string{TestIDKey, TestDeletionKey} {
		if _, found := spec[key]; found {
			t.Fatalf(ExpectedMissing, t.Name(), key)
		}
	}
}

func TestToViewWithoutSpec(t *testing.T) {
	cases := []struct {
		name string
		obj  *unstructured.Unstructured
	}{
		{"nil object", nil},
		{"no spec", &unstructured.Unstructured{Object: map[string]any{}}},
		{"spec not a map", &unstructured.Unstructured{Object: map[string]any{TestSpecKey: TestSizeValue}}},
	}
	for _, tc := range cases {
		if got := view.ToView(tc.obj, widgetMetadata(nil)); got != nil {
			t.Fatalf(ExpectedNilView, tc.name, got)
		}
	}
}

func TestSplitPatch(t *testing.T) {
	viewOnly := map[string]any{TestIDKey: TestName, TestMetadataKey: map[string]any{}, TestDeletionKey: TestDeletionStamp}
	cases := []struct {
		name         string
		statusFields map[string]string
		body         map[string]any
		wantSpec     map[string]any
		wantStatus   map[string]any
	}{
		{"no status fields routes all to spec", nil,
			map[string]any{TestSizeKey: TestSizeValue, TestPhaseKey: TestPhaseActive},
			map[string]any{TestSizeKey: TestSizeValue, TestPhaseKey: TestPhaseActive}, map[string]any{}},
		{"projected key goes to its status key", map[string]string{TestPhaseView: TestPhaseKey},
			map[string]any{TestSizeKey: TestSizeValue, TestPhaseView: TestPhaseActive},
			map[string]any{TestSizeKey: TestSizeValue}, map[string]any{TestPhaseKey: TestPhaseActive}},
		{"whole status view merges into status", map[string]string{TestWholeView: TestWholeStatusKey},
			map[string]any{TestWholeView: map[string]any{TestPhaseKey: TestPhaseDone}},
			map[string]any{}, map[string]any{TestPhaseKey: TestPhaseDone}},
		{"whole status view that is not a map is dropped", map[string]string{TestWholeView: TestWholeStatusKey},
			map[string]any{TestWholeView: TestPhaseDone}, map[string]any{}, map[string]any{}},
		{"view-only keys are dropped", map[string]string{TestPhaseKey: TestPhaseKey}, viewOnly,
			map[string]any{}, map[string]any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, status := view.SplitPatch(widgetMetadata(tc.statusFields), tc.body)
			if !reflect.DeepEqual(spec, tc.wantSpec) || !reflect.DeepEqual(status, tc.wantStatus) {
				t.Fatalf(ExpectedSplitLen, tc.name, spec, status, len(tc.wantSpec), len(tc.wantStatus))
			}
		})
	}
}
