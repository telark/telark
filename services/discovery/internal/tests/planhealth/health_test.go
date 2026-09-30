package planhealth

import (
	"context"
	"encoding/json"
	"testing"
	"time"

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
	healthPolicy  = "pol-a"
	labelErr      = "err"
	labelHealth   = "health"
	modeEnforce   = "Enforce"
	fieldValidate = "validate"
	fieldMessage  = "message"
	handEdit      = "edited by hand"
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

func firstRule(t *testing.T, live *unstructured.Unstructured) map[string]any {
	t.Helper()
	rules, _, _ := unstructured.NestedFieldNoCopy(live.Object, "spec", "rules")
	list, ok := rules.([]any)
	if !ok || len(list) == constants.DefaultInitValue {
		t.Fatal("policy has no rules")
	}
	rule, ok := list[constants.DefaultInitValue].(map[string]any)
	if !ok {
		t.Fatal("rule is not an object")
	}
	return rule
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
		ID:    "pp-abc-1234-5678",
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

// W4-plans-4: health compared only the render-hash annotation, so a hand-edited rule that kept it
// read healthy and was never repaired. A field the policy engine defaults is not drift.
func TestComputeComparesLiveRules(t *testing.T) {
	cases := []struct {
		name       string
		edit       func(rule map[string]any)
		wantHealth string
		wantStale  int
	}{
		{"defaulted field", func(rule map[string]any) { rule["skipBackgroundRequests"] = true }, plans.HealthHealthy, constants.DefaultInitValue},
		{"edited message", func(rule map[string]any) {
			setRuleField(t, rule, handEdit, fieldValidate, fieldMessage)
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"added rule-level audit", func(rule map[string]any) {
			setRuleField(t, rule, "Audit", fieldValidate, "failureAction")
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"existing violations allowed", func(rule map[string]any) {
			setRuleField(t, rule, true, fieldValidate, "allowExistingViolations")
		}, plans.HealthDrifted, constants.DefaultAddValue},
		{"appended exclude", func(rule map[string]any) {
			excludes, _, _ := unstructured.NestedSlice(rule, "exclude", "any")
			wildcard := map[string]any{"resources": map[string]any{"namespaces": []any{"*"}}}
			setRuleField(t, rule, append(excludes, wildcard), "exclude", "any")
		}, plans.HealthDrifted, constants.DefaultAddValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, _ := renderedPlan(t, plans.PhaseActive)
			fresh, err := dpolicies.Render(plan, nil, nil)
			testutil.Equal(t, "render", err, nil)
			live := roundTripped(t, liveObject(t, &fresh[constants.DefaultInitValue]))
			c.edit(firstRule(t, live))

			res, err := health.Compute(context.Background(), health.Deps{Dyn: fakeDyn(live)}, plan)
			testutil.Equal(t, labelErr, err, nil)
			testutil.Equal(t, labelHealth, res.Health, c.wantHealth)
			testutil.Equal(t, "stale", len(res.Stale), c.wantStale)
		})
	}
}

func setRuleField(t *testing.T, rule map[string]any, value any, fields ...string) {
	t.Helper()
	if err := unstructured.SetNestedField(rule, value, fields...); err != nil {
		t.Fatalf("edit: %v", err)
	}
}

// The edited rule is redeployed on the same tick, and the plan reads healthy again.
func TestComputeAndRepairRestoresEditedRule(t *testing.T) {
	plan, policyName := renderedPlan(t, plans.PhaseActive)
	fresh, err := dpolicies.Render(plan, nil, nil)
	testutil.Equal(t, "render", err, nil)
	want := fresh[constants.DefaultInitValue].Spec.Rules[constants.DefaultInitValue].Validation.Message
	live := roundTripped(t, liveObject(t, &fresh[constants.DefaultInitValue]))
	if err := unstructured.SetNestedField(firstRule(t, live), handEdit, fieldValidate, fieldMessage); err != nil {
		t.Fatalf("edit: %v", err)
	}
	dyn := withApplyCreate(fakeDyn(live))

	res, err := health.ComputeAndRepair(context.Background(), repairDeps(dyn, &recordingLogger{}), plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, labelHealth, res.Health, plans.HealthHealthy)
	got, _, _ := unstructured.NestedString(firstRule(t, getPolicy(t, dyn, policyName)), fieldValidate, fieldMessage)
	testutil.Equal(t, "message restored", got, want)
}
