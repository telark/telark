package planhealth

import (
	"context"
	"testing"

	"github.com/telark/data/plans"
	dpolicies "github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/health"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	healthPolicy = "pol-a"
	labelErr     = "err"
	labelHealth  = "health"
	modeEnforce  = "Enforce"
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
			"labels": map[string]any{
				dpolicies.LabelPlanID:    planID,
				dpolicies.LabelManagedBy: dpolicies.ManagedByValue,
			},
		},
		"spec":   map[string]any{"validationFailureAction": action},
		"status": status,
	}}
}

func fakeDyn(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
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
	plan := activePlan(healthPolicy)
	dyn := fakeDyn(kyvernoPolicy(healthPolicy, plan.ID, modeEnforce, true))
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthHealthy)
	testutil.Equal(t, "policies", len(res.Policies), constants.DefaultAddValue)
	testutil.Equal(t, "no missing", len(res.Missing), constants.DefaultInitValue)
}

// A present-but-not-ready policy degrades the plan.
func TestComputeDegraded(t *testing.T) {
	plan := activePlan(healthPolicy)
	dyn := fakeDyn(kyvernoPolicy(healthPolicy, plan.ID, modeEnforce, false))
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthDegraded)
}

// A rendered policy absent from the cluster drifts the plan and is reported as
// missing.
func TestComputeDriftedMissing(t *testing.T) {
	plan := activePlan(healthPolicy)
	res, err := health.Compute(context.Background(), fakeDyn(), plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthDrifted)
	testutil.Equal(t, "missing", len(res.Missing), constants.DefaultAddValue)
}

// An extra cluster policy the plan never rendered counts as drift via the
// unexpected set.
func TestComputeDriftedUnexpected(t *testing.T) {
	plan := activePlan(healthPolicy)
	dyn := fakeDyn(
		kyvernoPolicy(healthPolicy, plan.ID, modeEnforce, true),
		kyvernoPolicy(rogueName, plan.ID, modeEnforce, true),
	)
	res, err := health.Compute(context.Background(), dyn, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthDrifted)
	testutil.Equal(t, "unexpected", len(res.Unexpected), constants.DefaultAddValue)
}

// ToPatch always carries the health and timestamp pointers, and only attaches a
// detail list when the result has detail rows.
func TestToPatch(t *testing.T) {
	bare := health.ToPatch(health.Result{Health: plans.HealthHealthy}, "now")
	if bare.Health == nil || *bare.Health != plans.HealthHealthy {
		t.Fatalf("patch health = %v", bare.Health)
	}
	testutil.Equal(t, "no detail", len(bare.HealthDetail), constants.DefaultInitValue)

	withDetail := health.ToPatch(health.Result{
		Health: plans.HealthDrifted,
		Detail: []plans.ProtectionPlanHealthDetail{{PolicyName: healthPolicy, Present: true, Ready: true}},
	}, "now")
	testutil.Equal(t, "detail rows", len(withDetail.HealthDetail), constants.DefaultAddValue)
	testutil.Equal(t, "detail name", withDetail.HealthDetail[0].PolicyName, healthPolicy)
}
