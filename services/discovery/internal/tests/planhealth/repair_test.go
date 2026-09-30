package planhealth

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/telark/internal/data/plans"
	dpolicies "github.com/telark/telark/internal/data/policies"
	_ "github.com/telark/telark/internal/data/policies/templates" // registers the renderers Render needs
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/health"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	policyKind         = "Policy"
	policyResource     = "policy"
	repairPlanNS       = "prod"
	goneNS             = "gone"
	aliveApp           = "alive"
	doomedApp          = "doomed"
	kindDeployment     = "Deployment"
	guardPlan          = "guard"
	rogueName          = "pol-rogue"
	reconcilePlanCount = 3
)

type recordingLogger struct {
	infos  []string
	errors []string
}

func (l *recordingLogger) Info(msg string)  { l.infos = append(l.infos, msg) }
func (l *recordingLogger) Error(msg string) { l.errors = append(l.errors, msg) }

func policyMapper() meta.RESTMapper {
	gv := protpolicies.KyvernoPolicyGVR.GroupVersion()
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv})
	mapper.AddSpecific(
		gv.WithKind(policyKind),
		protpolicies.KyvernoPolicyGVR,
		gv.WithResource(policyResource),
		meta.RESTScopeNamespace,
	)
	return mapper
}

// The fake tracker rejects a server-side apply for an object that does not exist yet, so the
// create half of SSA is emulated here; the update half keeps the status the way the status
// subresource does on a real apply.
func withApplyCreate(dyn *dynamicfake.FakeDynamicClient) *dynamicfake.FakeDynamicClient {
	dyn.PrependReactor("patch", protpolicies.KyvernoPolicyGVR.Resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		if !ok || patch.GetPatchType() != types.ApplyPatchType {
			return false, nil, nil
		}
		obj := &unstructured.Unstructured{}
		if err := obj.UnmarshalJSON(patch.GetPatch()); err != nil {
			return true, nil, err
		}
		if err := dyn.Tracker().Create(protpolicies.KyvernoPolicyGVR, obj, patch.GetNamespace()); err != nil {
			existing, getErr := dyn.Tracker().Get(protpolicies.KyvernoPolicyGVR, patch.GetNamespace(), patch.GetName())
			if getErr != nil {
				return true, nil, getErr
			}
			if status, ok := existing.(*unstructured.Unstructured).Object["status"]; ok {
				obj.Object["status"] = status
			}
			return true, nil, dyn.Tracker().Update(protpolicies.KyvernoPolicyGVR, obj, patch.GetNamespace())
		}
		return true, obj, nil
	})
	return dyn
}

func repairDeps(dyn dynamic.Interface, logger health.Logger) health.Deps {
	return health.Deps{
		Dyn:      dyn,
		Applier:  protpolicies.NewApplier(dyn, policyMapper()),
		Logger:   logger,
		Exporter: &fakePlanStore{phase: plans.PhaseActive},
	}
}

type fakePlanStore struct {
	phase   string
	getErr  error
	mu      sync.Mutex
	patched map[string]string
}

func (f *fakePlanStore) Get(planID string) (*plans.ProtectionPlan, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &plans.ProtectionPlan{ID: planID, Phase: f.phase}, nil
}

func (f *fakePlanStore) PatchOrError(_, planID string, req planseps.PatchProtectionPlanRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.patched == nil {
		f.patched = map[string]string{}
	}
	if req.Health != nil {
		f.patched[planID] = *req.Health
	}
	return nil
}

func (f *fakePlanStore) health(planID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.patched[planID]
}

// The rendered name is derived from the plan, so the fixture renders once and reuses it
// instead of hardcoding a hash.
func renderedPlan(t *testing.T, phase string) (*plans.ProtectionPlan, string) {
	t.Helper()
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-5678",
		Name:     guardPlan,
		Phase:    phase,
		Mode:     plans.ModeEnforce,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{repairPlanNS}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}},
	}
	rendered, err := dpolicies.Render(plan, nil, nil)
	if err != nil {
		t.Fatalf("render fixture: %v", err)
	}
	if len(rendered) != constants.DefaultAddValue {
		t.Fatalf("render fixture = %d policies, want 1", len(rendered))
	}
	plan.RenderedPolicies = []string{rendered[constants.DefaultInitValue].Name}
	return plan, rendered[constants.DefaultInitValue].Name
}

func getPolicy(t *testing.T, dyn dynamic.Interface, name string) *unstructured.Unstructured {
	t.Helper()
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).
		Namespace(repairPlanNS).
		Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	return got
}

// An active plan is repaired in place: deleted policies come back, flipped modes are
// patched back, and policies the plan never rendered are removed.
type repairCase struct {
	name     string
	phase    string
	cluster  func(planID, policyName string) []runtime.Object
	wantPol  bool
	wantMode string
	wantLog  bool
	rogue    bool
}

func TestComputeAndRepair(t *testing.T) {
	cases := []repairCase{
		{
			name:    "missing is redeployed",
			phase:   plans.PhaseActive,
			cluster: func(string, string) []runtime.Object { return nil },
			wantPol: true,
			wantLog: true,
		},
		{
			name:  "mode mismatch is patched",
			phase: plans.PhaseActive,
			cluster: func(planID, policyName string) []runtime.Object {
				return []runtime.Object{kyvernoPolicy(policyName, planID, "Audit", true)}
			},
			wantPol:  true,
			wantMode: modeEnforce,
			wantLog:  true,
		},
		{
			name:  "unexpected is deleted",
			phase: plans.PhaseActive,
			cluster: func(planID, policyName string) []runtime.Object {
				return []runtime.Object{
					kyvernoPolicy(policyName, planID, modeEnforce, true),
					kyvernoPolicy(rogueName, planID, modeEnforce, true),
				}
			},
			wantPol: true,
			wantLog: true,
			rogue:   true,
		},
		{
			name:    "non active plan is left alone",
			phase:   plans.PhaseScheduled,
			cluster: func(string, string) []runtime.Object { return nil },
			wantPol: false,
			wantLog: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertRepairCase(t, c) })
	}
}

func assertRepairCase(t *testing.T, c repairCase) {
	t.Helper()
	plan, policyName := renderedPlan(t, c.phase)
	dyn := withApplyCreate(fakeDyn(c.cluster(plan.ID, policyName)...))
	logger := &recordingLogger{}

	result, err := health.ComputeAndRepair(context.Background(), repairDeps(dyn, logger), plan)
	testutil.Equal(t, labelErr, err, nil)
	if len(logger.errors) > constants.DefaultInitValue {
		t.Fatalf("repair errors: %v", logger.errors)
	}
	testutil.Equal(t, "repair logged", len(logger.infos) > 0, c.wantLog)

	got := getPolicy(t, dyn, policyName)
	testutil.Equal(t, "policy present", got != nil, c.wantPol)
	if c.wantMode != "" {
		action, _, _ := unstructured.NestedString(got.Object, "spec", "validationFailureAction")
		testutil.Equal(t, "failure action", action, c.wantMode)
	}
	if c.rogue {
		testutil.Equal(t, "rogue removed", getPolicy(t, dyn, rogueName) == nil, true)
		testutil.Equal(t, labelHealth, result.Health, plans.HealthHealthy)
	}
	testutil.Equal(t, "missing after repair", len(result.Missing), constants.DefaultInitValue)
}

// Cancel/Terminate delete the policies before the phase patch lands. A check that
// started while the plan was still Active must not redeploy them on top of a plan
// that is already terminal, or the policies outlive the plan with nothing to clean them.
func TestComputeAndRepairSkipsWhenPlanLeftActive(t *testing.T) {
	plan, policyName := renderedPlan(t, plans.PhaseActive)
	dyn := withApplyCreate(fakeDyn())
	logger := &recordingLogger{}
	deps := repairDeps(dyn, logger)
	deps.Exporter = &fakePlanStore{phase: plans.PhaseCanceled}

	result, err := health.ComputeAndRepair(context.Background(), deps, plan)
	if err != nil {
		t.Fatalf("ComputeAndRepair: %v", err)
	}

	testutil.Equal(t, "policy not resurrected", getPolicy(t, dyn, policyName) == nil, true)
	testutil.Equal(t, "drift still reported", result.Health, plans.HealthDrifted)
}

// A phase read that fails tells us nothing about whether the plan is still active, so the
// repair is skipped — but it must be reported, not swallowed into a silent no-op.
func TestComputeAndRepairReportsPhaseReadFailure(t *testing.T) {
	plan, policyName := renderedPlan(t, plans.PhaseActive)
	dyn := withApplyCreate(fakeDyn())
	logger := &recordingLogger{}
	deps := repairDeps(dyn, logger)
	deps.Exporter = &fakePlanStore{getErr: errors.New("exporter unavailable")}

	if _, err := health.ComputeAndRepair(context.Background(), deps, plan); err != nil {
		t.Fatalf("ComputeAndRepair: %v", err)
	}

	testutil.Equal(t, "policy not resurrected", getPolicy(t, dyn, policyName) == nil, true)
	testutil.Equal(t, "failure logged", len(logger.errors), constants.DefaultAddValue)
}

func resolvedApp(name, namespace string) dpolicies.ResolvedApp {
	return dpolicies.ResolvedApp{
		Namespaces: []string{namespace},
		Resources:  []dpolicies.ApplicationResourceRef{{Kind: kindDeployment, Name: name, Namespace: namespace}},
	}
}

func liveObject(t *testing.T, pol *kyvernov1.Policy) *unstructured.Unstructured {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pol)
	if err != nil {
		t.Fatalf("to unstructured: %v", err)
	}
	live := &unstructured.Unstructured{Object: obj}
	live.Object["status"] = map[string]any{"conditions": []any{map[string]any{"type": "Ready", "status": "True"}}}
	return live
}

// A policy left behind by an older renderer (no render hash, or one from another plan revision)
// is present, ready and in the right mode, yet its content is not what the plan renders now.
// The regression: health called it healthy, so an upgrade never re-rendered active plans.
func TestComputeAndRepairRedeploysStalePolicies(t *testing.T) {
	cases := []struct {
		name string
		hash string
	}{
		{"pre-upgrade policy without a hash", constants.EmptyString},
		{"policy from another plan revision", "0000"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, policyName := renderedPlan(t, plans.PhaseActive)
			old := kyvernoPolicy(policyName, plan.ID, modeEnforce, true)
			if c.hash != constants.EmptyString {
				old.SetAnnotations(map[string]string{dpolicies.AnnotationRenderHash: c.hash})
			}
			dyn := withApplyCreate(fakeDyn(old))
			logger := &recordingLogger{}
			deps := repairDeps(dyn, logger)

			before, err := health.Compute(context.Background(), deps, plan)
			testutil.Equal(t, labelErr, err, nil)
			if !slices.Equal(before.Stale, []string{policyName}) {
				t.Fatalf("stale before = %v, want [%s]", before.Stale, policyName)
			}
			testutil.Equal(t, "health before", before.Health, plans.HealthDrifted)

			after, err := health.ComputeAndRepair(context.Background(), deps, plan)
			testutil.Equal(t, labelErr, err, nil)
			testutil.Equal(t, "errors", len(logger.errors), constants.DefaultInitValue)
			testutil.Equal(t, "health after", after.Health, plans.HealthHealthy)
			testutil.Equal(t, "stale after", len(after.Stale), constants.DefaultInitValue)

			fresh, err := dpolicies.Render(plan, nil, nil)
			testutil.Equal(t, "render", err, nil)
			want := fresh[constants.DefaultInitValue].Annotations[dpolicies.AnnotationRenderHash]
			testutil.Equal(t, "live hash", getPolicy(t, dyn, policyName).GetAnnotations()[dpolicies.AnnotationRenderHash], want)
		})
	}
}

// The regression: an application deleted with its namespace made every tick fail the repair
// with "applications not found" and left the plan drifted for good. Its policy is neither drift
// nor repairable, so the check covers the applications that are still there.
func TestComputeAndRepairSkipsVanishedApplication(t *testing.T) {
	both := map[string]dpolicies.ResolvedApp{
		aliveApp:  resolvedApp(aliveApp, repairPlanNS),
		doomedApp: resolvedApp(doomedApp, goneNS),
	}
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-9999",
		Name:     guardPlan,
		Phase:    plans.PhaseActive,
		Mode:     plans.ModeAudit,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{aliveApp, doomedApp}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "block-update"}},
	}
	rendered, err := dpolicies.Render(plan, both, nil)
	testutil.Equal(t, "render", err, nil)
	testutil.Equal(t, "one policy per application", len(rendered), len(both))
	for i := range rendered {
		plan.RenderedPolicies = append(plan.RenderedPolicies, rendered[i].Name)
	}
	alive := slices.IndexFunc(rendered, func(p kyvernov1.Policy) bool { return p.Namespace == repairPlanNS })

	dyn := withApplyCreate(fakeDyn(liveObject(t, &rendered[alive])))
	logger := &recordingLogger{}
	deps := repairDeps(dyn, logger)
	deps.ResolveApps = func(context.Context, []string) (map[string]dpolicies.ResolvedApp, []string, error) {
		return map[string]dpolicies.ResolvedApp{aliveApp: both[aliveApp]}, []string{doomedApp}, nil
	}

	result, err := health.ComputeAndRepair(context.Background(), deps, plan)
	testutil.Equal(t, labelErr, err, nil)
	if len(logger.errors)+len(logger.infos) > constants.DefaultInitValue {
		t.Fatalf("vanished application logged: %v %v", logger.errors, logger.infos)
	}
	testutil.Equal(t, labelHealth, result.Health, plans.HealthHealthy)
	testutil.Equal(t, "rows", len(result.Policies), constants.DefaultAddValue)
	testutil.Equal(t, "row", result.Policies[constants.DefaultInitValue].Name, rendered[alive].Name)
}

// A plan activated by a renderer that covered only an application's first namespace lists one
// policy; the current renderer produces one per namespace. The regression: the second one was
// never deployed and health stayed healthy, so only cancel and reactivate closed the gap.
func TestComputeAndRepairDeploysPoliciesTheRenderNowProduces(t *testing.T) {
	app := dpolicies.ResolvedApp{
		Namespaces: []string{repairPlanNS, goneNS},
		Resources: []dpolicies.ApplicationResourceRef{
			{Kind: kindDeployment, Name: aliveApp, Namespace: repairPlanNS},
			{Kind: kindDeployment, Name: aliveApp, Namespace: goneNS},
		},
	}
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-7777",
		Name:     guardPlan,
		Phase:    plans.PhaseActive,
		Mode:     plans.ModeEnforce,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{aliveApp}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "block-update"}},
	}
	resolved := map[string]dpolicies.ResolvedApp{aliveApp: app}
	rendered, err := dpolicies.Render(plan, resolved, nil)
	testutil.Equal(t, "render", err, nil)
	testutil.Equal(t, "one policy per namespace", len(rendered), len(app.Namespaces))
	first := slices.IndexFunc(rendered, func(p kyvernov1.Policy) bool { return p.Namespace == repairPlanNS })
	second := rendered[len(rendered)-constants.DefaultAddValue-first]
	plan.RenderedPolicies = []string{rendered[first].Name}

	dyn := withApplyCreate(fakeDyn(liveObject(t, &rendered[first])))
	logger := &recordingLogger{}
	deps := repairDeps(dyn, logger)
	deps.ResolveApps = func(context.Context, []string) (map[string]dpolicies.ResolvedApp, []string, error) {
		return resolved, nil, nil
	}

	result, err := health.ComputeAndRepair(context.Background(), deps, plan)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "errors", len(logger.errors), constants.DefaultInitValue)
	// Kyverno has not stamped Ready on the new policy yet, so the pass reads degraded, not drifted.
	testutil.Equal(t, "added after repair", len(result.Added), constants.DefaultInitValue)
	testutil.Equal(t, "missing after repair", len(result.Missing), constants.DefaultInitValue)
	if !slices.Equal(result.Rendered, []string{rendered[first].Name, second.Name}) {
		t.Fatalf("rendered set = %v, want both namespaces", result.Rendered)
	}
	testutil.Equal(t, "second namespace patched", len(health.ToPatch(result, "now").RenderedPolicies), len(app.Namespaces))
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(goneNS).Get(context.Background(), second.Name, metav1.GetOptions{})
	testutil.Equal(t, "second policy deployed", err, nil)
	testutil.Equal(t, "second policy hash", got.GetAnnotations()[dpolicies.AnnotationRenderHash], second.Annotations[dpolicies.AnnotationRenderHash])
}

// The reconcile pass reads the cluster once for every plan it covers: a per-plan selector
// made each tick cost one cluster-wide Policy LIST per active plan.
func TestReconcileForActiveListsTheClusterOnce(t *testing.T) {
	planList := make([]plans.ProtectionPlan, reconcilePlanCount)
	objs := make([]runtime.Object, reconcilePlanCount)
	for i := range planList {
		planID := fmt.Sprintf("pp-abc-1234-%04d", i)
		name := fmt.Sprintf("pol-%d", i)
		planList[i] = plans.ProtectionPlan{
			ID:               planID,
			Phase:            plans.PhaseActive,
			Mode:             plans.ModeEnforce,
			RenderedPolicies: []string{name},
		}
		objs[i] = kyvernoPolicy(name, planID, modeEnforce, true)
	}

	dyn := fakeDyn(objs...)
	var lists atomic.Int64
	dyn.PrependReactor("list", protpolicies.KyvernoPolicyGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		lists.Add(1)
		return false, nil, nil
	})

	store := &fakePlanStore{phase: plans.PhaseActive}
	deps := repairDeps(dyn, &recordingLogger{})
	deps.Exporter = store
	deps.Clock = func() time.Time { return time.Unix(0, 0).UTC() }

	health.ReconcileForActive(context.Background(), deps, planList)

	testutil.Equal(t, "cluster lists", lists.Load(), int64(constants.DefaultAddValue))
	for i := range planList {
		testutil.Equal(t, "health "+planList[i].ID, store.health(planList[i].ID), plans.HealthHealthy)
	}
}
