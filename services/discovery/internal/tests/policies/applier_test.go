package policies

import (
	"context"
	"testing"

	datapolicies "github.com/telark/data/policies"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func policyObj(name, ns, planID, action string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1",
		"kind":       "Policy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": ns,
			"labels":    map[string]any{datapolicies.LabelPlanID: planID},
		},
		"spec": map[string]any{"validationFailureAction": action},
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
		policyObj("pol-a", "prod", "pp-1", "Enforce"),
		policyObj("pol-b", "stage", "pp-1", "Enforce"),
		policyObj("pol-other", "prod", "pp-2", "Enforce"),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.CleanupByPlanID(context.Background(), "pp-1"); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	testutil.Equal(t, "remaining", countPolicies(t, dyn), 1)
}

// DeletePoliciesByLabelAndNames deletes only the named policies within the plan
// label, leaving the rest.
func TestApplierDeleteByLabelAndNames(t *testing.T) {
	dyn := applierDyn(
		policyObj("pol-a", "prod", "pp-1", "Enforce"),
		policyObj("pol-b", "prod", "pp-1", "Enforce"),
		policyObj("pol-c", "prod", "pp-1", "Enforce"),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.DeletePoliciesByLabelAndNames(context.Background(), "pp-1", []string{"pol-a", "pol-c"}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	testutil.Equal(t, "remaining", countPolicies(t, dyn), 1)

	// An empty name list is a no-op.
	testutil.Equal(t, "noop", applier.DeletePoliciesByLabelAndNames(context.Background(), "pp-1", nil), nil)
}

// PatchPoliciesMode rewrites the failure action on every labelled policy.
func TestApplierPatchPoliciesMode(t *testing.T) {
	dyn := applierDyn(policyObj("pol-a", "prod", "pp-1", "Enforce"))
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.PatchPoliciesMode(context.Background(), "pp-1", "audit"); err != nil {
		t.Fatalf("patch: %v", err)
	}
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace("prod").Get(context.Background(), "pol-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	action, _, _ := unstructured.NestedString(got.Object, "spec", "validationFailureAction")
	if action == "Enforce" {
		t.Fatalf("failure action was not patched, still %q", action)
	}

	// No matching policies is a no-op success.
	testutil.Equal(t, "no match", applier.PatchPoliciesMode(context.Background(), "pp-absent", "audit"), nil)
}

// DeletePoliciesByNamespacedName removes exactly the referenced policies.
func TestApplierDeleteByNamespacedName(t *testing.T) {
	dyn := applierDyn(
		policyObj("pol-a", "prod", "pp-1", "Enforce"),
		policyObj("pol-b", "stage", "pp-1", "Enforce"),
	)
	applier := protpolicies.NewApplier(dyn, nil)
	err := applier.DeletePoliciesByNamespacedName(context.Background(), []protpolicies.NamespacedName{
		{Namespace: "prod", Name: "pol-a"},
	})
	if err != nil {
		t.Fatalf("delete by ns/name: %v", err)
	}
	testutil.Equal(t, "remaining", countPolicies(t, dyn), 1)
}
