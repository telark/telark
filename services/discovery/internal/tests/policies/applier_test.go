package policies

import (
	"context"
	"errors"
	"testing"

	datapolicies "github.com/telark/telark/internal/data/policies"
	"github.com/telark/telark/services/discovery/internal/constants"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	planIDOne       = "pp-1"
	policyNamespace = "prod"
	policyA         = "pol-a"
	policyB         = "pol-b"
	modeEnforce     = "Enforce"
	labelRemaining  = "remaining"
)

func policyObj(name, ns, planID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1",
		"kind":       "Policy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
			"labels":    map[string]any{datapolicies.LabelPlanID: planID},
		},
		"spec": map[string]any{"validationFailureAction": modeEnforce},
	}}
}

func applierDyn(objs ...runtime.Object) dynamic.Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{
		protpolicies.KyvernoPolicyGVR: "PolicyList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)
}

func countPolicies(t *testing.T, dyn dynamic.Interface) int {
	t.Helper()
	list, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}
	return len(list.Items)
}

// CleanupByPlanID deletes every policy carrying the plan's label across all
// namespaces.
func TestApplierCleanupByPlanID(t *testing.T) {
	dyn := applierDyn(
		policyObj(policyA, policyNamespace, planIDOne),
		policyObj(policyB, "stage", planIDOne),
		policyObj("pol-other", policyNamespace, "pp-2"),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.CleanupByPlanID(context.Background(), planIDOne); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	testutil.Equal(t, labelRemaining, countPolicies(t, dyn), constants.DefaultAddValue)
}

// DeletePoliciesByLabelAndNames deletes only the named policies within the plan
// label, leaving the rest.
func TestApplierDeleteByLabelAndNames(t *testing.T) {
	dyn := applierDyn(
		policyObj(policyA, policyNamespace, planIDOne),
		policyObj(policyB, policyNamespace, planIDOne),
		policyObj("pol-c", policyNamespace, planIDOne),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.DeletePoliciesByLabelAndNames(context.Background(), planIDOne, []string{policyA, "pol-c"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	testutil.Equal(t, labelRemaining, countPolicies(t, dyn), constants.DefaultAddValue)

	// An empty name list is a no-op.
	testutil.Equal(t, "noop", applier.DeletePoliciesByLabelAndNames(context.Background(), planIDOne, nil), nil)
}

// PatchPoliciesMode rewrites the failure action on every labeled policy.
func TestApplierPatchPoliciesMode(t *testing.T) {
	dyn := applierDyn(policyObj(policyA, policyNamespace, planIDOne))
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.PatchPoliciesMode(context.Background(), planIDOne, "audit"); err != nil {
		t.Fatalf("patch: %v", err)
	}
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(policyNamespace).Get(context.Background(), policyA, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	action, _, _ := unstructured.NestedString(got.Object, "spec", "validationFailureAction")
	if action == modeEnforce {
		t.Fatalf("failure action was not patched, still %q", action)
	}

	// No matching policies is a no-op success.
	testutil.Equal(t, "no match", applier.PatchPoliciesMode(context.Background(), "pp-absent", "audit"), nil)
}

// DeletePoliciesByNamespacedName removes exactly the referenced policies.
func TestApplierDeleteByNamespacedName(t *testing.T) {
	dyn := applierDyn(
		policyObj(policyA, policyNamespace, planIDOne),
		policyObj(policyB, "stage", planIDOne),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	err := applier.DeletePoliciesByNamespacedName(context.Background(), []protpolicies.NamespacedName{
		{Namespace: policyNamespace, Name: policyA},
	})
	if err != nil {
		t.Fatalf("delete by ns/name: %v", err)
	}
	testutil.Equal(t, labelRemaining, countPolicies(t, dyn), constants.DefaultAddValue)
}

// A delete failure other than NotFound is returned instead of being swallowed, so callers can
// tell a real cleanup failure from a completed one.
func TestApplierDeleteErrorsPropagate(t *testing.T) {
	refs := []protpolicies.NamespacedName{{Namespace: policyNamespace, Name: policyA}}
	cases := []struct {
		name  string
		run   func(*protpolicies.Applier) error
		fails bool
	}{
		{"cleanup by plan", func(a *protpolicies.Applier) error {
			return a.CleanupByPlanID(context.Background(), planIDOne)
		}, true},
		{"delete by label and names", func(a *protpolicies.Applier) error {
			return a.DeletePoliciesByLabelAndNames(context.Background(), planIDOne, []string{policyA})
		}, true},
		{"delete by namespaced name", func(a *protpolicies.Applier) error {
			return a.DeletePoliciesByNamespacedName(context.Background(), refs)
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dyn := applierDyn(policyObj(policyA, policyNamespace, planIDOne))
			fake, ok := dyn.(*dynamicfake.FakeDynamicClient)
			if !ok {
				t.Fatal("fake dynamic client expected")
			}
			fake.PrependReactor("delete", "policies", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("boom")
			})
			err := c.run(protpolicies.NewApplier(dyn, nil))
			testutil.Equal(t, "failed", err != nil, c.fails)
		})
	}
}

// A policy already gone is the state the caller asked for, so NotFound stays a success.
func TestApplierDeleteIgnoresNotFound(t *testing.T) {
	dyn := applierDyn()
	applier := protpolicies.NewApplier(dyn, nil)
	err := applier.DeletePoliciesByNamespacedName(context.Background(), []protpolicies.NamespacedName{
		{Namespace: policyNamespace, Name: "absent"},
	})
	testutil.Equal(t, "not found ignored", err, nil)
}
