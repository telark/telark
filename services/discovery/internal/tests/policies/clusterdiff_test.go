package policies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	datapolicies "github.com/telark/telark/internal/data/policies"
	"github.com/telark/telark/services/discovery/internal/constants"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	fullRenderTemplate = "block-delete"
	fullRenderPlanName = "full"
	excludedKind       = "ConfigMap"
	newPolicyName      = "new-name"
	stalePolicyName    = "stale-name"
	policyKind         = "Policy"
	applyFailures      = 1
	labelNoErr         = "no error"
	labelApplied       = "applied"
)

type noopDiffLogger struct{}

func (noopDiffLogger) Info(string)  {}
func (noopDiffLogger) Error(string) {}

func diffMsgs() protpolicies.DeployErrorMessages {
	return protpolicies.DeployErrorMessages{
		GenericDeployFailure: "deploy failed",
		InternalErrorFormat:  "internal: %s",
	}
}

// A plan that is not active is a no-op: the current rendered policies are
// returned unchanged and nothing touches the cluster.
func TestApplyClusterDiffInactivePlan(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: planIDOne, Phase: "pending", Mode: plans.ModeEnforce, RenderedPolicies: []string{policyA}}
	applier := protpolicies.NewApplier(applierDyn(), nil)
	deployed, kept, stale, err := protpolicies.ApplyClusterDiff(
		context.Background(), applier, noopDiffLogger{}, plan, nil, nil, nil, plan.Mode, diffMsgs(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "nothing deployed", len(deployed), constants.DefaultInitValue)
	testutil.Equal(t, "kept rendered", len(kept), constants.DefaultAddValue)
	testutil.Equal(t, "nothing stale", len(stale), constants.DefaultInitValue)
}

// An active plan with no deploy or remove combos and an unchanged mode keeps the
// rendered policies and reports no error.
func TestApplyClusterDiffNoCombos(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: planIDOne, Phase: plans.PhaseActive, Mode: plans.ModeEnforce, RenderedPolicies: []string{policyA, policyB}}
	applier := protpolicies.NewApplier(applierDyn(policyObj(policyA, policyNamespace, planIDOne)), nil)
	deployed, kept, stale, err := protpolicies.ApplyClusterDiff(
		context.Background(), applier, noopDiffLogger{}, plan, nil, nil, nil, plan.Mode, diffMsgs(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "nothing deployed", len(deployed), constants.DefaultInitValue)
	testutil.Equal(t, "kept both", len(kept), constants.TwoValue)
	testutil.Equal(t, "nothing stale", len(stale), constants.DefaultInitValue)
}

// The regression: removals were deleted before the exporter patch, so a refused patch left the
// plan without them until the next tick. They now come back as stale and stay live; a name the
// edit renders again is never stale.
func TestApplyClusterDiffDefersRemovals(t *testing.T) {
	name := renderedName(t, fullRenderPlan(plans.PhaseActive))
	plan := fullRenderPlan(plans.PhaseActive, name, policyA)
	removed := protpolicies.Combinations(plan.Policies, plan.Scope.Namespaces)
	cases := []struct {
		name       string
		deploy     []protpolicies.PolicyTargetCombo
		wantStale  []string
		wantKept   []string
		wantDeploy int
	}{
		{"removed name deferred", nil, []string{name}, []string{policyA}, constants.DefaultInitValue},
		{"re-rendered name kept", removed, []string{}, []string{name, policyA}, constants.DefaultAddValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			applier, fake, _ := recordingApplier(t, constants.DefaultInitValue, policyObj(name, policyNamespace, planIDOne))
			deployed, kept, stale, err := protpolicies.ApplyClusterDiff(
				context.Background(), applier, noopDiffLogger{}, plan, c.deploy, removed, nil, plan.Mode, diffMsgs(),
			)
			testutil.Equal(t, labelNoErr, err, nil)
			testutil.Equal(t, "deployed", len(deployed), c.wantDeploy)
			if !slices.Equal(stale, c.wantStale) || !slices.Equal(kept, c.wantKept) {
				t.Fatalf("stale=%v kept=%v, want %v %v", stale, kept, c.wantStale, c.wantKept)
			}
			testutil.Equal(t, "removal still live", slices.Equal(livePolicyNames(t, fake), []string{name}), true)
		})
	}
}

// RollbackPatchFailure is a no-op when nothing was deployed, and deletes the
// just-deployed policies otherwise.
func TestRollbackPatchFailure(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: planIDOne}
	dyn := applierDyn(policyObj(policyA, policyNamespace, planIDOne))
	applier := protpolicies.NewApplier(dyn, nil)

	protpolicies.RollbackPatchFailure(context.Background(), applier, noopDiffLogger{}, plan, nil, plan.Mode)
	testutil.Equal(t, "noop keeps policy", countPolicies(t, dyn), constants.DefaultAddValue)

	protpolicies.RollbackPatchFailure(context.Background(), applier, noopDiffLogger{}, plan, []string{policyA}, plan.Mode)
	testutil.Equal(t, "rolled back", countPolicies(t, dyn), constants.DefaultInitValue)
}

// The regression: the diff path switched the live mode before the exporter patch and a refused
// patch left it switched until the next tick.
func TestRollbackPatchFailureRestoresMode(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: planIDOne, Mode: plans.ModeEnforce}
	dyn := applierDyn(policyObj(policyA, policyNamespace, planIDOne))
	applier := protpolicies.NewApplier(dyn, nil)
	if err := applier.PatchPoliciesMode(context.Background(), planIDOne, plans.ModeAudit); err != nil {
		t.Fatalf("patch: %v", err)
	}

	protpolicies.RollbackPatchFailure(context.Background(), applier, noopDiffLogger{}, plan, nil, plans.ModeAudit)
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(policyNamespace).Get(context.Background(), policyA, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	action, _, _ := unstructured.NestedString(got.Object, "spec", "validationFailureAction")
	testutil.Equal(t, "failure action", action, modeEnforce)
}

type applyRecorder struct {
	failures int
	applied  []map[string]any
}

// The object tracker Gets before it applies, so it cannot create a policy on apply: the reactor
// records every applied object instead, and fails the first `failures` applies.
func (r *applyRecorder) react(action k8stesting.Action) (bool, runtime.Object, error) {
	patch, ok := action.(k8stesting.PatchAction)
	if !ok || patch.GetPatchType() != types.ApplyPatchType {
		return false, nil, nil
	}
	if r.failures > constants.DefaultInitValue {
		r.failures--
		return true, nil, errors.New("apply failed")
	}
	obj := map[string]any{}
	if err := json.Unmarshal(patch.GetPatch(), &obj); err != nil {
		return true, nil, err
	}
	r.applied = append(r.applied, obj)
	return true, nil, nil
}

func (r *applyRecorder) names() []string {
	out := make([]string, constants.DefaultInitValue, len(r.applied))
	for _, obj := range r.applied {
		out = append(out, (&unstructured.Unstructured{Object: obj}).GetName())
	}
	return out
}

// Deploy dereferences the mapper, so these tests map kyverno.io/v1 Policy to its GVR.
func recordingApplier(
	t *testing.T,
	failures int,
	objs ...runtime.Object,
) (*protpolicies.Applier, *dynamicfake.FakeDynamicClient, *applyRecorder) {
	t.Helper()
	fake, ok := applierDyn(objs...).(*dynamicfake.FakeDynamicClient)
	if !ok {
		t.Fatal("fake dynamic client expected")
	}
	rec := &applyRecorder{failures: failures}
	fake.PrependReactor("patch", "policies", rec.react)
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(protpolicies.KyvernoPolicyGVR.GroupVersion().WithKind(policyKind), meta.RESTScopeNamespace)
	return protpolicies.NewApplier(fake, mapper), fake, rec
}

func livePolicyNames(t *testing.T, fake *dynamicfake.FakeDynamicClient) []string {
	t.Helper()
	list, err := fake.Resource(protpolicies.KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list live policies: %v", err)
	}
	out := make([]string, constants.DefaultInitValue, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].GetName())
	}
	slices.Sort(out)
	return out
}

// Every rule of the applied policy carries an exclude.any entry for exactly the excluded kind.
func excludesKind(obj map[string]any) bool {
	rules, _, _ := unstructured.NestedSlice(obj, "spec", "rules")
	return len(rules) > constants.DefaultInitValue && !slices.ContainsFunc(rules, func(rule any) bool {
		return !ruleExcludesKind(rule)
	})
}

func ruleExcludesKind(rule any) bool {
	fields, ok := rule.(map[string]any)
	if !ok {
		return false
	}
	filters, _, _ := unstructured.NestedSlice(fields, "exclude", "any")
	return slices.ContainsFunc(filters, func(filter any) bool {
		entry, ok := filter.(map[string]any)
		if !ok {
			return false
		}
		kinds, _, _ := unstructured.NestedStringSlice(entry, "resources", "kinds")
		return slices.Equal(kinds, []string{excludedKind})
	})
}

func fullRenderPlan(phase string, rendered ...string) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:               planIDOne,
		Name:             fullRenderPlanName,
		Phase:            phase,
		Mode:             plans.ModeEnforce,
		Scope:            plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{policyNamespace}},
		Policies:         []plans.ProtectionPlanPolicy{{TemplateID: fullRenderTemplate}},
		RenderedPolicies: rendered,
	}
}

func withExclusions(plan *plans.ProtectionPlan) *plans.ProtectionPlan {
	next := *plan
	next.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{excludedKind}}
	return &next
}

// Exclusions never change names, so the old and the new plan render this same single name.
func renderedName(t *testing.T, plan *plans.ProtectionPlan) string {
	t.Helper()
	rendered, err := datapolicies.Render(plan, nil, nil)
	if err != nil || len(rendered) != constants.DefaultAddValue {
		t.Fatalf("render = %d policies, err %v; want exactly one", len(rendered), err)
	}
	return rendered[constants.DefaultInitValue].Name
}

func TestApplyFullRenderInactivePlan(t *testing.T) {
	old := fullRenderPlan(plans.PhaseScheduled, policyA)
	applier, fake, _ := recordingApplier(t, constants.DefaultInitValue)
	deployed, kept, stale, err := protpolicies.ApplyFullRender(
		context.Background(), applier, noopDiffLogger{}, old, withExclusions(old), nil, diffMsgs(),
	)
	testutil.Equal(t, labelNoErr, err, nil)
	testutil.Equal(t, "deployed", len(deployed), constants.DefaultInitValue)
	testutil.Equal(t, "stale", len(stale), constants.DefaultInitValue)
	if !slices.Equal(kept, old.RenderedPolicies) {
		t.Fatalf("kept = %v, want the old rendered %v", kept, old.RenderedPolicies)
	}
	testutil.Equal(t, "cluster calls", len(fake.Actions()), constants.DefaultInitValue)
}

// The new render is deployed and the old names it did not reproduce come back as stale. They are
// never deleted here: Run deletes them only after the exporter patch lands.
func TestApplyFullRenderDeploysAndReturnsStale(t *testing.T) {
	old := fullRenderPlan(plans.PhaseActive, stalePolicyName)
	next := withExclusions(old)
	name := renderedName(t, next)
	applier, fake, rec := recordingApplier(t, constants.DefaultInitValue, policyObj(stalePolicyName, policyNamespace, planIDOne))
	deployed, kept, stale, err := protpolicies.ApplyFullRender(context.Background(), applier, noopDiffLogger{}, old, next, nil, diffMsgs())
	testutil.Equal(t, labelNoErr, err, nil)
	if !slices.Equal(deployed, []string{name}) || kept != nil || !slices.Equal(stale, []string{stalePolicyName}) {
		t.Fatalf("deployed=%v kept=%v stale=%v; want [%s] nil [%s]", deployed, kept, stale, name, stalePolicyName)
	}
	if !slices.Equal(livePolicyNames(t, fake), []string{stalePolicyName}) {
		t.Fatal("the stale policy must survive until the exporter patch lands")
	}
	if !slices.Equal(rec.names(), []string{name}) || !slices.ContainsFunc(rec.applied, excludesKind) {
		t.Fatalf("applied %v, want [%s] carrying the exclusion", rec.names(), name)
	}
}

// A failed deploy keeps the plan's live policy and re-applies it from the old plan, because SSA
// may already have overwritten it with the new exclusions.
func TestApplyFullRenderDeployFailureKeepsExisting(t *testing.T) {
	name := renderedName(t, fullRenderPlan(plans.PhaseActive))
	old := fullRenderPlan(plans.PhaseActive, name)
	applier, fake, rec := recordingApplier(t, applyFailures, policyObj(name, policyNamespace, planIDOne))
	_, _, _, err := protpolicies.ApplyFullRender(context.Background(), applier, noopDiffLogger{}, old, withExclusions(old), nil, diffMsgs())
	msgs := diffMsgs()
	if want := fmt.Sprintf(msgs.InternalErrorFormat, msgs.GenericDeployFailure); err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	if !slices.Equal(livePolicyNames(t, fake), []string{name}) {
		t.Fatal("the plan's live policy must survive a failed deploy")
	}
	if !slices.Equal(rec.names(), []string{name}) || slices.ContainsFunc(rec.applied, excludesKind) {
		t.Fatalf("applied %v, want [%s] re-applied from the old plan without the exclusion", rec.names(), name)
	}
}

// The patch-failure path: names the old plan never had are deleted, the old names survive and are
// re-applied from the old plan. Nothing attempted means nothing touched.
func TestRollbackFullRenderKeepsExisting(t *testing.T) {
	oldName := renderedName(t, fullRenderPlan(plans.PhaseActive))
	old := fullRenderPlan(plans.PhaseActive, oldName)
	seed := func() []runtime.Object {
		return []runtime.Object{
			policyObj(oldName, policyNamespace, planIDOne),
			policyObj(newPolicyName, policyNamespace, planIDOne),
		}
	}

	applier, fake, rec := recordingApplier(t, constants.DefaultInitValue, seed()...)
	protpolicies.RollbackFullRender(context.Background(), applier, noopDiffLogger{}, old, []string{oldName, newPolicyName}, nil)
	if !slices.Equal(livePolicyNames(t, fake), []string{oldName}) {
		t.Fatalf("live = %v, want only %s", livePolicyNames(t, fake), oldName)
	}
	if !slices.Equal(rec.names(), []string{oldName}) {
		t.Fatalf("re-applied %v, want [%s]", rec.names(), oldName)
	}

	applier, fake, rec = recordingApplier(t, constants.DefaultInitValue, seed()...)
	protpolicies.RollbackFullRender(context.Background(), applier, noopDiffLogger{}, old, nil, nil)
	testutil.Equal(t, "nothing deleted", countPolicies(t, fake), constants.TwoValue)
	testutil.Equal(t, labelApplied, len(rec.applied), constants.DefaultInitValue)
}

// The combos path runs only when exclusions are unchanged, so every sub-plan it renders must carry
// the stored exclusions.
func TestRenderForCombinationsCarriesExclusions(t *testing.T) {
	plan := withExclusions(fullRenderPlan(plans.PhaseActive))
	applier, _, rec := recordingApplier(t, constants.DefaultInitValue)
	deployed, _, _, err := protpolicies.ApplyClusterDiff(
		context.Background(), applier, noopDiffLogger{}, plan,
		protpolicies.Combinations(plan.Policies, plan.Scope.Namespaces), nil, nil, plan.Mode, diffMsgs(),
	)
	testutil.Equal(t, labelNoErr, err, nil)
	testutil.Equal(t, labelApplied, len(rec.applied), len(deployed))
	if len(deployed) == constants.DefaultInitValue || slices.ContainsFunc(rec.applied, func(obj map[string]any) bool {
		return !excludesKind(obj)
	}) {
		t.Fatalf("applied %v, want every policy carrying the stored exclusion", rec.names())
	}
}
