package planupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/telark/data/plans"
	dpolicies "github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers the renderers Run deploys with
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/helpers/telarkconfig"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	reportseps "github.com/telark/rest/endpoints/reports"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	runPlanID      = "pp-abc-1234-5678"
	runNS          = "prod"
	liveApp        = "web"
	vanishedApp    = "gone"
	vanishedNS     = "legacy"
	tplBlockCreate = "block-create"
	tplBlockDelete = "block-delete"
	tplBlockUpdate = "block-update"
	kindDeployment = "Deployment"
	policyKind     = "Policy"
	editedText     = "edited"
	labelErr       = "err"
	labelPatches   = "exporter patches"
	startedAt      = "2026-01-15T00:00:00Z"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type nopLogger struct{}

func (nopLogger) Info(string)  {}
func (nopLogger) Error(string) {}

// Serves the plan on every read and answers every patch with patchStatus.
type exporterStub struct {
	plan        plans.ProtectionPlan
	patchStatus int
	patches     []map[string]any
}

func (e *exporterStub) install(t *testing.T) {
	t.Helper()
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			return answer(t, http.StatusOK, e.plan), nil
		}
		body := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode patch: %v", err)
		}
		e.patches = append(e.patches, body)
		return answer(t, e.patchStatus, nil), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}

func answer(t *testing.T, status int, data any) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": status, "message": "m", "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
}

type capturedRun struct {
	plan    plans.ProtectionPlan
	endedAt string
	actor   string
	reason  string
	trigger string
}

// Records the report hooks, and which policies were still live when the checkpoint ran.
type reportsRecorder struct {
	dyn         *dynamicfake.FakeDynamicClient
	captures    []capturedRun
	checkpoints [][]string
	liveAtCheck []string
}

func (r *reportsRecorder) capture(plan *plans.ProtectionPlan, endedAt, actor, reason, trigger string) <-chan struct{} {
	r.captures = append(r.captures, capturedRun{plan: *plan, endedAt: endedAt, actor: actor, reason: reason, trigger: trigger})
	done := make(chan struct{})
	close(done)
	return done
}

func (r *reportsRecorder) checkpoint(ctx context.Context, plan *plans.ProtectionPlan) (*reports.PlanReportLedger, error) {
	r.checkpoints = append(r.checkpoints, slices.Clone(plan.RenderedPolicies))
	r.liveAtCheck = liveNames(ctx, r.dyn)
	return &reports.PlanReportLedger{}, nil
}

func liveNames(ctx context.Context, dyn *dynamicfake.FakeDynamicClient) []string {
	list, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	out := make([]string, constants.DefaultInitValue, len(list.Items))
	for i := range list.Items {
		out = append(out, list.Items[i].GetName())
	}
	slices.Sort(out)
	return out
}

// The tracker cannot create on a server-side apply, so an apply is acknowledged and dropped.
func clusterWith(t *testing.T, live ...runtime.Object) *dynamicfake.FakeDynamicClient {
	t.Helper()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{protpolicies.KyvernoPolicyGVR: "PolicyList"}, live...,
	)
	dyn.PrependReactor("patch", protpolicies.KyvernoPolicyGVR.Resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		return ok && patch.GetPatchType() == types.ApplyPatchType, nil, nil
	})
	return dyn
}

func runDeps(dyn *dynamicfake.FakeDynamicClient, rec *reportsRecorder) update.Deps {
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(protpolicies.KyvernoPolicyGVR.GroupVersion().WithKind(policyKind), meta.RESTScopeNamespace)
	rec.dyn = dyn
	return update.Deps{
		Applier:     protpolicies.NewApplier(dyn, mapper),
		Exporter:    clients.NewProtectionPlanClient(),
		ResolveApps: resolveOnlyLiveApp,
		Logger:      nopLogger{},
		Clock:       func() time.Time { return editNow },
		Activate:    func(context.Context, *plans.ProtectionPlan) error { return nil },
		CaptureRun:  rec.capture,
		Checkpoint:  rec.checkpoint,
	}
}

func resolveOnlyLiveApp(_ context.Context, ids []string) (map[string]dpolicies.ResolvedApp, []string, error) {
	resolved := map[string]dpolicies.ResolvedApp{}
	var missing []string
	for _, id := range ids {
		if id != liveApp {
			missing = append(missing, id)
			continue
		}
		resolved[id] = dpolicies.ResolvedApp{
			Namespaces: []string{runNS},
			Resources:  []dpolicies.ApplicationResourceRef{{Kind: kindDeployment, Name: liveApp, Namespace: runNS}},
		}
	}
	return resolved, missing, nil
}

// An active namespaces plan whose rendered policies are all live.
func activeNamespacesPlan(t *testing.T, mode string, templates ...string) (*plans.ProtectionPlan, []runtime.Object) {
	t.Helper()
	plan := &plans.ProtectionPlan{
		ID:        runPlanID,
		Name:      "guard",
		Severity:  plans.SeverityLow,
		Phase:     plans.PhaseActive,
		Mode:      mode,
		TimeMode:  plans.TimeModePermanent,
		StartedAt: strptr(startedAt),
		Scope:     plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{runNS}},
	}
	for _, tpl := range templates {
		plan.Policies = append(plan.Policies, plans.ProtectionPlanPolicy{TemplateID: tpl})
	}
	rendered, err := dpolicies.Render(plan, nil, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	live := make([]runtime.Object, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		obj, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(&rendered[i])
		if convErr != nil {
			t.Fatalf("to unstructured: %v", convErr)
		}
		plan.RenderedPolicies = append(plan.RenderedPolicies, rendered[i].Name)
		live = append(live, &unstructured.Unstructured{Object: obj})
	}
	telarkconfig.SetExcludedForTest([]string{})
	return plan, live
}

func requestFor(plan *plans.ProtectionPlan) *planseps.PrepareProtectionPlanRequest {
	req := &planseps.PrepareProtectionPlanRequest{
		Name:     plan.Name,
		Severity: plan.Severity,
		Mode:     plan.Mode,
		TimeMode: plan.TimeMode,
		Scope: planseps.ScopeRequest{
			Type: plan.Scope.Type, Namespaces: plan.Scope.Namespaces, ApplicationRefs: plan.Scope.ApplicationRefs,
		},
	}
	for _, p := range plan.Policies {
		req.Policies = append(req.Policies, planseps.PolicyRequest{TemplateID: p.TemplateID})
	}
	return req
}

func failureAction(t *testing.T, dyn *dynamicfake.FakeDynamicClient, name string) string {
	t.Helper()
	got, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(runNS).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	action, _, _ := unstructured.NestedString(got.Object, "spec", "validationFailureAction")
	return action
}

// W4-plans-5: an empty namespace list, or namespaces on an applications plan, passed the edit
// checks and deleted live policies before the exporter refused the patch with a 500.
func TestRunRejectsInvalidScopeBeforeTouchingTheCluster(t *testing.T) {
	cases := []struct {
		name  string
		scope func(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest)
	}{
		{"empty namespaces", func(_ *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest) {
			req.Scope.Namespaces = []string{}
		}},
		{"namespaces on an applications plan", func(plan *plans.ProtectionPlan, req *planseps.PrepareProtectionPlanRequest) {
			plan.Scope = plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{liveApp}}
			req.Scope = planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{liveApp}, Namespaces: []string{runNS}}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
			req := requestFor(plan)
			c.scope(plan, req)
			stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
			stub.install(t)
			dyn := clusterWith(t, live...)

			_, err := update.Run(context.Background(), runDeps(dyn, &reportsRecorder{}), userID, plan.ID, req)
			testutil.Equal(t, "validation error", validation.IsValidation(err), true)
			testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultInitValue)
			testutil.Equal(t, "cluster calls", len(dyn.Actions()), constants.DefaultInitValue)
		})
	}
}

// W4-plans-5 and W4-plans-3: a withdrawn template's policy stays live until the exporter patch
// lands, its denials are checkpointed first, and a refused patch restores the switched mode.
func TestRunWithdrawsPoliciesOnlyAfterThePatch(t *testing.T) {
	cases := []struct {
		name        string
		patchStatus int
		wantErr     bool
		wantLive    int
		wantChecks  int
	}{
		{"patch lands", http.StatusOK, false, constants.DefaultAddValue, constants.DefaultAddValue},
		{"patch refused", http.StatusBadRequest, true, constants.TwoValue, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan, live := activeNamespacesPlan(t, plans.ModeEnforce, tplBlockCreate, tplBlockDelete)
			req := requestFor(plan)
			req.Policies = req.Policies[:constants.DefaultAddValue]
			req.Mode = plans.ModeAudit
			stub := &exporterStub{plan: *plan, patchStatus: c.patchStatus}
			stub.install(t)
			dyn := clusterWith(t, live...)
			rec := &reportsRecorder{}

			_, err := update.Run(context.Background(), runDeps(dyn, rec), userID, plan.ID, req)
			testutil.Equal(t, labelErr, err != nil, c.wantErr)
			testutil.Equal(t, "live policies", len(liveNames(context.Background(), dyn)), c.wantLive)
			testutil.Equal(t, "checkpoints", len(rec.checkpoints), c.wantChecks)
			if c.wantChecks > constants.DefaultInitValue {
				testutil.Equal(t, "checkpoint saw the withdrawn policy", slices.Equal(rec.checkpoints[0], plan.RenderedPolicies), true)
				testutil.Equal(t, "withdrawn policy live at checkpoint", len(rec.liveAtCheck), constants.TwoValue)
			}
			if c.wantErr {
				want := string(dpolicies.FailureAction(plans.ModeEnforce))
				testutil.Equal(t, "mode restored", failureAction(t, dyn, plan.RenderedPolicies[0]), want)
			}
		})
	}
}

// W4-plans-2: parking an active plan dropped its run without a report, and the next activation
// started a new ledger.
func TestRunParkCapturesTheEndedRun(t *testing.T) {
	plan, live := activeNamespacesPlan(t, plans.ModeAudit, tplBlockCreate)
	req := requestFor(plan)
	req.TimeMode = plans.TimeModeTimeRange
	req.TimeRange = window(futureStart, futureEnd)
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	dyn := clusterWith(t, live...)
	rec := &reportsRecorder{}

	_, err := update.Run(context.Background(), runDeps(dyn, rec), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "parked", stub.patches[0][protection.FieldPhase], any(plans.PhaseScheduled))
	testutil.Equal(t, "policies withdrawn", len(liveNames(context.Background(), dyn)), constants.DefaultInitValue)
	testutil.Equal(t, "captures", len(rec.captures), constants.DefaultAddValue)
	got := rec.captures[constants.DefaultInitValue]
	testutil.Equal(t, "ended at the edit", got.endedAt, editNow.Format(globalshared.DefaultTimeFormat))
	testutil.Equal(t, "actor", got.actor, userID)
	testutil.Equal(t, "reason", got.reason, protection.ReasonParked)
	testutil.Equal(t, "trigger", got.trigger, reportseps.TriggerEnd)
	testutil.Equal(t, "captured phase", got.plan.Phase, plans.PhaseActive)
	testutil.Equal(t, "captured run", *got.plan.StartedAt, startedAt)
	testutil.Equal(t, "captured policies", slices.Equal(got.plan.RenderedPolicies, plan.RenderedPolicies), true)
}

// W4-plans-6: a plan referencing a vanished application could not be edited at all. Only a target
// the edit deploys has to exist.
func TestRunToleratesVanishedApplications(t *testing.T) {
	cases := []struct {
		name    string
		phase   string
		refs    []string
		newRefs []string
		wantErr bool
		patches int
		window  *planseps.TimeRangeRequest
	}{
		{"description edit of an active plan", plans.PhaseActive, []string{vanishedApp}, []string{vanishedApp},
			false, constants.DefaultAddValue, nil},
		{"retarget of an active plan", plans.PhaseActive, []string{vanishedApp}, []string{liveApp}, false, constants.DefaultAddValue, nil},
		{"description edit of a failed plan", plans.PhaseFailed, []string{vanishedApp}, []string{vanishedApp}, false, constants.DefaultAddValue, nil},
		{"added target must exist", plans.PhaseActive, []string{liveApp}, []string{liveApp, vanishedApp}, true, constants.DefaultInitValue, nil},
		{"activating edit needs every target", plans.PhaseScheduled, []string{vanishedApp}, []string{vanishedApp},
			true, constants.DefaultInitValue, nil},
		// The window opening later would otherwise fail the plan with nothing enforced.
		{"added target of a scheduled plan must exist", plans.PhaseScheduled, []string{liveApp}, []string{liveApp, vanishedApp},
			true, constants.DefaultInitValue, window(futureStart, futureEnd)},
		{"added target of a failed plan must exist", plans.PhaseFailed, []string{liveApp}, []string{liveApp, vanishedApp},
			true, constants.DefaultInitValue, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := &plans.ProtectionPlan{
				ID:        runPlanID,
				Name:      "guard",
				Severity:  plans.SeverityLow,
				Phase:     c.phase,
				Mode:      plans.ModeAudit,
				TimeMode:  plans.TimeModePermanent,
				StartedAt: strptr(startedAt),
				Scope:     plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: c.refs},
				Policies:  []plans.ProtectionPlanPolicy{{TemplateID: tplBlockUpdate}},
			}
			if c.phase == plans.PhaseScheduled {
				plan.TimeMode = plans.TimeModeTimeRange
				plan.TimeRange = &plans.ProtectionPlanTimeRange{StartAt: futureStart, EndAt: futureEnd}
			}
			req := requestFor(plan)
			req.TimeMode = plans.TimeModePermanent
			if c.window != nil {
				req.TimeMode, req.TimeRange = plans.TimeModeTimeRange, c.window
			}
			req.Scope.ApplicationRefs = c.newRefs
			req.Description = strptr(editedText)
			stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
			stub.install(t)

			_, err := update.Run(context.Background(), runDeps(clusterWith(t), &reportsRecorder{}), userID, plan.ID, req)
			testutil.Equal(t, labelErr, err != nil, c.wantErr)
			if c.wantErr {
				testutil.Equal(t, "validation error", validation.IsValidation(err), true)
			}
			testutil.Equal(t, labelPatches, len(stub.patches), c.patches)
		})
	}
}

// D6: removing a vanished application from an active plan left its live policy enforcing and listed.
func TestRunWithdrawsARemovedVanishedApplication(t *testing.T) {
	plan := &plans.ProtectionPlan{
		ID:        runPlanID,
		Name:      "guard",
		Severity:  plans.SeverityLow,
		Phase:     plans.PhaseActive,
		Mode:      plans.ModeAudit,
		TimeMode:  plans.TimeModePermanent,
		StartedAt: strptr(startedAt),
		Scope:     plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{liveApp, vanishedApp}},
		Policies:  []plans.ProtectionPlanPolicy{{TemplateID: tplBlockUpdate}},
	}
	resolved, _, err := resolveOnlyLiveApp(context.Background(), []string{liveApp})
	if err != nil {
		t.Fatal(err)
	}
	resolved[vanishedApp] = dpolicies.ResolvedApp{
		Namespaces: []string{vanishedNS},
		Resources:  []dpolicies.ApplicationResourceRef{{Kind: kindDeployment, Name: vanishedApp, Namespace: vanishedNS}},
	}
	rendered, err := dpolicies.Render(plan, resolved, nil)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	live := make([]runtime.Object, constants.DefaultInitValue, len(rendered))
	var kept string
	for i := range rendered {
		obj, convErr := runtime.DefaultUnstructuredConverter.ToUnstructured(&rendered[i])
		if convErr != nil {
			t.Fatalf("to unstructured: %v", convErr)
		}
		plan.RenderedPolicies = append(plan.RenderedPolicies, rendered[i].Name)
		live = append(live, &unstructured.Unstructured{Object: obj})
		if rendered[i].Namespace == runNS {
			kept = rendered[i].Name
		}
	}
	telarkconfig.SetExcludedForTest([]string{})
	req := requestFor(plan)
	req.Scope.ApplicationRefs = []string{liveApp}
	stub := &exporterStub{plan: *plan, patchStatus: http.StatusOK}
	stub.install(t)
	dyn := clusterWith(t, live...)

	_, err = update.Run(context.Background(), runDeps(dyn, &reportsRecorder{}), userID, plan.ID, req)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "vanished policy withdrawn", slices.Equal(liveNames(context.Background(), dyn), []string{kept}), true)
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultAddValue)
	listed, ok := stub.patches[constants.DefaultInitValue][protection.FieldRenderedPolicies].([]any)
	testutil.Equal(t, "vanished policy unlisted", ok && slices.Equal(listed, []any{kept}), true)
}
