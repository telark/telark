package planhealth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	dpolicies "github.com/telark/telark/internal/data/policies"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/health"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	healthPolicy  = "pol-a"
	labelErr      = "err"
	labelHealth   = "health"
	modeEnforce   = "Enforce"
	modeAudit     = "Audit"
	actionDeny    = "Deny"
	fieldMessage  = "message"
	handEdit      = "edited by hand"
	fieldStatus   = "status"
	fieldSpec     = "spec"
	fixturePlanID = "pp-abc-1234-5678"
)

func policyMeta(name, planID string) map[string]any {
	return map[string]any{
		"name":      name,
		"namespace": "prod",
		"labels": map[string]any{
			dpolicies.LabelPlanID:    planID,
			dpolicies.LabelManagedBy: dpolicies.ManagedByValue,
		},
	}
}

func kyvernoPolicy(name, planID, mode string, ready bool) *unstructured.Unstructured {
	action := modeAudit
	if mode == modeEnforce {
		action = actionDeny
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": protpolicies.KyvernoPolicyGVR.GroupVersion().String(),
		"kind":       dpolicies.PolicyKind,
		"metadata":   policyMeta(name, planID),
		fieldSpec:    map[string]any{"validationActions": []any{action}},
		fieldStatus:  map[string]any{"conditionStatus": map[string]any{"ready": ready}},
	}}
}

func fakeDyn(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	gvrToListKind := map[schema.GroupVersionResource]string{
		protpolicies.KyvernoPolicyGVR: "NamespacedValidatingPolicyList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, objs...)
}

func activePlan(policies ...string) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:               fixturePlanID,
		Phase:            plans.PhaseActive,
		Mode:             plans.ModeEnforce,
		RenderedPolicies: policies,
	}
}

// Plans outside the active phase, or active plans with nothing rendered, resolve
// without ever consulting the cluster.
func TestComputeShortCircuits(t *testing.T) {
	pending := &plans.ProtectionPlan{Phase: "pending"}
	res, err := health.Compute(context.Background(), health.Deps{}, pending)
	testutil.Equal(t, "pending err", err, nil)
	testutil.Equal(t, "pending health", res.Health, plans.HealthUnknown)

	empty := activePlan()
	res, err = health.Compute(context.Background(), health.Deps{}, empty)
	testutil.Equal(t, "empty err", err, nil)
	testutil.Equal(t, "empty health", res.Health, plans.HealthDrifted)
}

// A single rendered policy that is present, ready and matches the enforce mode is
// healthy.
func TestComputeHealthy(t *testing.T) {
	plan := activePlan(healthPolicy)
	dyn := fakeDyn(kyvernoPolicy(healthPolicy, plan.ID, modeEnforce, true))
	res, err := health.Compute(context.Background(), health.Deps{Dyn: dyn}, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthHealthy)
	testutil.Equal(t, "policies", len(res.Policies), constants.DefaultAddValue)
	testutil.Equal(t, "no missing", len(res.Missing), constants.DefaultInitValue)
}

// A present-but-not-ready policy degrades the plan.
func TestComputeDegraded(t *testing.T) {
	plan := activePlan(healthPolicy)
	dyn := fakeDyn(kyvernoPolicy(healthPolicy, plan.ID, modeEnforce, false))
	res, err := health.Compute(context.Background(), health.Deps{Dyn: dyn}, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthDegraded)
}

// A rendered policy absent from the cluster drifts the plan and is reported as
// missing.
func TestComputeDriftedMissing(t *testing.T) {
	plan := activePlan(healthPolicy)
	res, err := health.Compute(context.Background(), health.Deps{Dyn: fakeDyn()}, plan)
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
	res, err := health.Compute(context.Background(), health.Deps{Dyn: dyn}, plan)
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

// Every non-active transition already stores health=unknown, so a status poll on such a plan
// must not PATCH it again: that only churned lastUpdatedAt/By=system on every poll.
func TestCheckSkipsPatchForNonActive(t *testing.T) {
	store := &fakePlanStore{phase: plans.PhaseCanceled}
	deps := health.Deps{
		Exporter: store,
		Logger:   &recordingLogger{},
		Clock:    func() time.Time { return time.Unix(0, 0).UTC() },
	}

	plan, res, err := health.Check(context.Background(), deps, healthPolicy)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "result health", res.Health, plans.HealthUnknown)
	testutil.Equal(t, "plan health", plan.Health, plans.HealthUnknown)
	testutil.Equal(t, "patched health", store.health(healthPolicy), constants.EmptyString)
}

// The live copy goes through JSON the way the API server and the dynamic client hand it back.
func roundTripped(t *testing.T, live *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	raw, err := json.Marshal(live.Object)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := &unstructured.Unstructured{}
	if err := out.UnmarshalJSON(raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func liveSpec(t *testing.T, live *unstructured.Unstructured) map[string]any {
	t.Helper()
	spec, ok := live.Object[fieldSpec].(map[string]any)
	if !ok {
		t.Fatal("policy has no spec")
	}
	return spec
}

func firstValidation(t *testing.T, live *unstructured.Unstructured) map[string]any {
	t.Helper()
	list, ok := liveSpec(t, live)["validations"].([]any)
	if !ok || len(list) == constants.DefaultInitValue {
		t.Fatal("policy has no validations")
	}
	validation, ok := list[constants.DefaultInitValue].(map[string]any)
	if !ok {
		t.Fatal("validation is not an object")
	}
	return validation
}

// Every template reads healthy once deployed, or each tick would redeploy it for nothing.
func TestComputeTrustsEveryDeployedTemplate(t *testing.T) {
	app := dpolicies.ResolvedApp{
		Namespaces: []string{repairPlanNS},
		Resources: []dpolicies.ApplicationResourceRef{
			{Kind: kindDeployment, Name: aliveApp, Namespace: repairPlanNS},
			{Kind: "ConfigMap", Name: aliveApp, Namespace: repairPlanNS},
			{Kind: "Secret", Name: aliveApp, Namespace: repairPlanNS},
		},
		VolumeClaims: []string{aliveApp},
	}
	resolved := map[string]dpolicies.ResolvedApp{aliveApp: app}
	plan := &plans.ProtectionPlan{
		ID:    fixturePlanID,
		Name:  guardPlan,
		Phase: plans.PhaseActive,
		Mode:  plans.ModeEnforce,
		Scope: plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{aliveApp}},
	}
	for _, tpl := range plans.Templates {
		params := map[string]any{}
		for _, spec := range tpl.Params {
			params[spec.Key] = []any{"latest"}
		}
		plan.Policies = append(plan.Policies, plans.ProtectionPlanPolicy{TemplateID: tpl.ID, Params: params})
	}
	rendered, err := dpolicies.Render(plan, resolved, nil)
	testutil.Equal(t, "render", err, nil)
	live := make([]runtime.Object, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		plan.RenderedPolicies = append(plan.RenderedPolicies, rendered[i].Name)
		live = append(live, roundTripped(t, liveObject(t, &rendered[i])))
	}
	testutil.Equal(t, "every template rendered", len(rendered) >= len(plans.Templates), true)
	deps := health.Deps{
		Dyn: fakeDyn(live...),
		ResolveApps: func(context.Context, []string) (map[string]dpolicies.ResolvedApp, []string, error) {
			return resolved, nil, nil
		},
	}

	res, err := health.Compute(context.Background(), deps, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthHealthy)
	testutil.Equal(t, "stale", len(res.Stale), constants.DefaultInitValue)
}

// W4-plans-4: health compared only the render-hash annotation, so a hand edit that kept it read
// healthy and was never repaired. What Kyverno writes into status is not drift.
func TestComputeComparesLiveSpec(t *testing.T) {
	cases := []struct {
		name       string
		edit       func(spec map[string]any)
		wantHealth string
		wantStale  int
	}{
		{"status written by Kyverno", func(map[string]any) {}, plans.HealthHealthy, constants.DefaultInitValue},
		{"edited message", func(spec map[string]any) {
			setField(t, spec["validations"].([]any)[constants.DefaultInitValue].(map[string]any), handEdit, fieldMessage)
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"added failure policy", func(spec map[string]any) {
			setField(t, spec, "Ignore", "failurePolicy")
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"background scans turned on", func(spec map[string]any) {
			setField(t, spec, true, "evaluation", "background", "enabled")
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"appended match condition", func(spec map[string]any) {
			conditions, _, _ := unstructured.NestedSlice(spec, "matchConditions")
			setField(t, spec, append(conditions, map[string]any{"name": "never", "expression": "false"}), "matchConditions")
		}, plans.HealthDrifted, constants.DefaultAddValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, _ := renderedPlan(t, plans.PhaseActive)
			fresh, err := dpolicies.Render(plan, nil, nil)
			testutil.Equal(t, "render", err, nil)
			live := roundTripped(t, liveObject(t, &fresh[constants.DefaultInitValue]))
			live.Object[fieldStatus] = map[string]any{"conditionStatus": map[string]any{"ready": true, "message": ""}, "generated": false}
			c.edit(liveSpec(t, live))

			res, err := health.Compute(context.Background(), health.Deps{Dyn: fakeDyn(live)}, plan)
			testutil.Equal(t, labelErr, err, nil)
			testutil.Equal(t, labelHealth, res.Health, c.wantHealth)
			testutil.Equal(t, "stale", len(res.Stale), c.wantStale)
		})
	}
}

func setField(t *testing.T, obj map[string]any, value any, fields ...string) {
	t.Helper()
	if err := unstructured.SetNestedField(obj, value, fields...); err != nil {
		t.Fatalf("edit: %v", err)
	}
}

// The edited rule is redeployed on the same tick, and the plan reads healthy again.
func TestComputeAndRepairRestoresEditedRule(t *testing.T) {
	plan, policyName := renderedPlan(t, plans.PhaseActive)
	fresh, err := dpolicies.Render(plan, nil, nil)
	testutil.Equal(t, "render", err, nil)
	want := fresh[constants.DefaultInitValue].Spec.Validations[constants.DefaultInitValue].Message
	live := roundTripped(t, liveObject(t, &fresh[constants.DefaultInitValue]))
	setField(t, firstValidation(t, live), handEdit, fieldMessage)
	dyn := withApplyCreate(fakeDyn(live))

	res, err := health.ComputeAndRepair(context.Background(), repairDeps(dyn, &recordingLogger{}), plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthHealthy)
	got, _, _ := unstructured.NestedString(firstValidation(t, getPolicy(t, dyn, policyName)), fieldMessage)
	testutil.Equal(t, "message restored", got, want)
}

// A cluster without the policies.kyverno.io CRDs can deploy nothing: the plan reads Degraded, the
// check still answers, and the repair never tries.
func TestComputeWithoutPolicyKind(t *testing.T) {
	plan := activePlan(healthPolicy)
	dyn := fakeDyn()
	dyn.PrependReactor("list", protpolicies.KyvernoPolicyGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8serrors.NewNotFound(protpolicies.KyvernoPolicyGVR.GroupResource(), "")
	})
	logger := &recordingLogger{}
	deps := repairDeps(dyn, logger)
	deps.Clock = func() time.Time { return sweepNow }

	res, err := health.ComputeAndRepair(context.Background(), deps, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthDegraded)
	testutil.Equal(t, "repair attempted", len(logger.infos)+len(logger.errors), constants.DefaultInitValue)

	store := &fakePlanStore{phase: plans.PhaseActive}
	deps.Exporter = store
	health.ReconcileForActive(context.Background(), deps, []plans.ProtectionPlan{*plan})
	testutil.Equal(t, "patched health", store.health(plan.ID), plans.HealthDegraded)
	testutil.Equal(t, "reason logged", len(logger.errors), constants.DefaultAddValue)
}
