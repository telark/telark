package planhealth

import (
	"context"
	"testing"

	dpolicies "github.com/telark/data/policies"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/health"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func kyvernoPolicy(name, planID, action string, ready bool) *unstructured.Unstructured {
	status := map[string]any{}
	if ready {
		status["conditions"] = []any{map[string]any{"type": "Ready", "status": "True"}}
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "kyverno.io/v1",
		"kind":       "Policy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "prod",
			"labels":    map[string]any{dpolicies.LabelPlanID: planID},
		},
		"spec":   map[string]any{"validationFailureAction": action},
		"status": status,
	}}
}

func fakeDyn(objs ...runtime.Object) dynamic.Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{
		protpolicies.KyvernoPolicyGVR: "PolicyList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)
}

func activePlan(policies ...string) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:               "pp-abc-1234-5678",
		Phase:            plans.PhaseActive,
		Mode:             plans.ModeEnforce,
		RenderedPolicies: policies,
	}
}

// Plans outside the active phase, or active plans with nothing rendered, resolve
// without ever consulting the cluster.
func TestComputeShortCircuits(t *testing.T) {
	pending := &plans.ProtectionPlan{Phase: "pending"}
	res, err := health.Compute(context.Background(), nil, pending)
	testutil.Equal(t, "pending err", err, nil)
	testutil.Equal(t, "pending health", res.Health, plans.HealthUnknown)

	empty := activePlan()
	res, err = health.Compute(context.Background(), nil, empty)
	testutil.Equal(t, "empty err", err, nil)
	testutil.Equal(t, "empty health", res.Health, plans.HealthDrifted)
}

// A single rendered policy that is present, ready and matches the enforce mode is
// healthy.
func TestComputeHealthy(t *testing.T) {
	plan := activePlan("pol-a")
	dyn := fakeDyn(kyvernoPolicy("pol-a", plan.ID, "Enforce", true))
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "health", res.Health, plans.HealthHealthy)
	testutil.Equal(t, "policies", len(res.Policies), 1)
	testutil.Equal(t, "no missing", len(res.Missing), 0)
}

// A present-but-not-ready policy degrades the plan.
func TestComputeDegraded(t *testing.T) {
	plan := activePlan("pol-a")
	dyn := fakeDyn(kyvernoPolicy("pol-a", plan.ID, "Enforce", false))
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "health", res.Health, plans.HealthDegraded)
}

// A rendered policy absent from the cluster drifts the plan and is reported as
// missing.
func TestComputeDriftedMissing(t *testing.T) {
	plan := activePlan("pol-a")
	res, err := health.Compute(context.Background(), fakeDyn(), plan)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "health", res.Health, plans.HealthDrifted)
	testutil.Equal(t, "missing", len(res.Missing), 1)
}

// An extra cluster policy the plan never rendered counts as drift via the
// unexpected set.
func TestComputeDriftedUnexpected(t *testing.T) {
	plan := activePlan("pol-a")
	dyn := fakeDyn(
		kyvernoPolicy("pol-a", plan.ID, "Enforce", true),
		kyvernoPolicy("pol-rogue", plan.ID, "Enforce", true),
	)
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "health", res.Health, plans.HealthDrifted)
	testutil.Equal(t, "unexpected", len(res.Unexpected), 1)
}

// ToPatch always carries the health and timestamp pointers, and only attaches a
// detail list when the result has detail rows.
func TestToPatch(t *testing.T) {
	bare := health.ToPatch(health.Result{Health: plans.HealthHealthy}, "now")
	if bare.Health == nil || *bare.Health != plans.HealthHealthy {
		t.Fatalf("patch health = %v", bare.Health)
	}
	testutil.Equal(t, "no detail", len(bare.HealthDetail), 0)

	withDetail := health.ToPatch(health.Result{
		Health: plans.HealthDrifted,
		Detail: []plans.ProtectionPlanHealthDetail{{PolicyName: "pol-a", Present: true, Ready: true}},
	}, "now")
	testutil.Equal(t, "detail rows", len(withDetail.HealthDetail), 1)
	testutil.Equal(t, "detail name", withDetail.HealthDetail[0].PolicyName, "pol-a")
}
