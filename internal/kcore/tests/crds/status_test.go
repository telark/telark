package crds

import (
	"context"
	"net/http"
	"testing"

	"github.com/telark/data/metadata/base"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/shared"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func widgetMetadata(statusFields map[string]string) base.Metadata {
	return base.Metadata{
		BaseGroup: TestGroup, Kind: TestKind, Version: TestVersion,
		Plural: TestPlural, Namespace: TestNamespace, StatusFields: statusFields,
	}
}

func widget(spec, status map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{TestSpecKey: spec}}
	if status != nil {
		obj.Object[TestStatusKey] = status
	}
	obj.SetAPIVersion(TestGroup + "/" + TestVersion)
	obj.SetKind(TestKind)
	obj.SetName(TestName)
	obj.SetNamespace(TestNamespace)
	return obj
}

func installFakeClient(t *testing.T, objects ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{widgetGVR: TestListKind}, objects...)
	k8sclient.SetDynamicClient(client)
	t.Cleanup(k8sclient.ResetAllClients)
	return client
}

func countStatusPatches(client *dynamicfake.FakeDynamicClient) int {
	count := NoPatches
	for _, action := range client.Actions() {
		if action.GetVerb() == TestVerbPatch && action.GetSubresource() == TestSubresource {
			count += OnePatch
		}
	}
	return count
}

func storedPhase(t *testing.T, client *dynamicfake.FakeDynamicClient) any {
	t.Helper()
	obj, err := client.Resource(widgetGVR).Namespace(TestNamespace).Get(context.Background(), TestName, k8smetav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	phase, _, _ := unstructured.NestedFieldNoCopy(obj.Object, TestStatusKey, TestPhaseKey)
	return phase
}

func assertStatus(t *testing.T, name string, got shared.KubernetesAPIData, want int) {
	t.Helper()
	if got.Status != want {
		t.Fatalf(ExpectedStatus, name, got.Status, got.Error, want)
	}
}

func TestPatchCustomResourceStatus(t *testing.T) {
	client := installFakeClient(t, widget(map[string]any{TestSizeKey: TestSizeValue}, nil))
	payload := map[string]any{TestStatusKey: map[string]any{TestPhaseKey: TestPhaseActive}}

	got := api.PatchCustomResourceStatus(widgetMetadata(nil), TestName, payload)

	assertStatus(t, t.Name(), got, http.StatusOK)
	if n := countStatusPatches(client); n != OnePatch {
		t.Fatalf(ExpectedActions, t.Name(), n, OnePatch)
	}
	if phase := storedPhase(t, client); phase != TestPhaseActive {
		t.Fatalf(ExpectedField, t.Name(), TestPhaseKey, phase, TestPhaseActive)
	}
}

func TestPatchCustomResourceStatusEmptyName(t *testing.T) {
	installFakeClient(t)
	got := api.PatchCustomResourceStatus(widgetMetadata(nil), "", map[string]any{})
	assertStatus(t, t.Name(), got, http.StatusBadRequest)
}

func TestUpdateCustomResourceStatus(t *testing.T) {
	client := installFakeClient(t, widget(map[string]any{TestSizeKey: TestSizeValue}, nil))
	var sawSubresource bool
	client.PrependReactor(TestVerbUpdate, TestPlural, func(action k8stesting.Action) (bool, runtime.Object, error) {
		sawSubresource = sawSubresource || action.GetSubresource() == TestSubresource
		return false, nil, nil
	})

	got := api.UpdateCustomResourceStatus(TestName, widgetMetadata(nil),
		widget(map[string]any{TestSizeKey: TestSizeValue}, map[string]any{TestPhaseKey: TestPhaseDone}))

	assertStatus(t, t.Name(), got, http.StatusOK)
	if !sawSubresource {
		t.Fatalf(ExpectedField, t.Name(), TestSubresource, sawSubresource, true)
	}
	if phase := storedPhase(t, client); phase != TestPhaseDone {
		t.Fatalf(ExpectedField, t.Name(), TestPhaseKey, phase, TestPhaseDone)
	}
}

func TestCreateCustomResourceWithStatus(t *testing.T) {
	gr := schema.GroupResource{Group: TestGroup, Resource: TestPlural}
	projected := map[string]string{TestPhaseView: TestPhaseKey}
	status := map[string]any{TestPhaseKey: TestPhaseActive}

	cases := []struct {
		name         string
		statusFields map[string]string
		status       map[string]any
		patchErrs    []error
		createErr    error
		wantStatus   int
		wantPatches  int
	}{
		{"plain create without status fields", nil, status, nil, nil, http.StatusOK, NoPatches},
		{"plain create without status in template", projected, nil, nil, nil, http.StatusOK, NoPatches},
		{"create then status patch", projected, status, nil, nil, http.StatusOK, OnePatch},
		{"status patch retries a not-found", projected, status, []error{k8serrors.NewNotFound(gr, TestName)}, nil, http.StatusOK, TwoPatches},
		{"status patch gives up on a permanent error", projected, status,
			[]error{k8serrors.NewForbidden(gr, TestName, nil)}, nil, http.StatusInternalServerError, OnePatch},
		{"create conflict skips status patch", projected, status, nil,
			k8serrors.NewAlreadyExists(gr, TestName), http.StatusConflict, NoPatches},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := installFakeClient(t)
			if tc.createErr != nil {
				client.PrependReactor(TestVerbCreate, TestPlural, failingReactor(tc.createErr))
			}
			pending := tc.patchErrs
			client.PrependReactor(TestVerbPatch, TestPlural, func(k8stesting.Action) (bool, runtime.Object, error) {
				if len(pending) == NoPatches {
					return false, nil, nil
				}
				err := pending[0]
				pending = pending[1:]
				return true, nil, err
			})

			got := api.CreateCustomResourceWithStatus(
				widget(map[string]any{TestSizeKey: TestSizeValue}, tc.status), widgetMetadata(tc.statusFields))

			assertStatus(t, tc.name, got, tc.wantStatus)
			if n := countStatusPatches(client); n != tc.wantPatches {
				t.Fatalf(ExpectedActions, tc.name, n, tc.wantPatches)
			}
		})
	}
}
