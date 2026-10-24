package planlifecycle

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	dpolicies "github.com/telark/telark/internal/data/policies"
	_ "github.com/telark/telark/internal/data/policies/templates" // registers the renderers approve and reactivate check
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
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
	approverID    = "u-approver"
	templateID    = "block-create"
	zoneUTC       = "UTC"
	clockLayout   = "15:04"
	windowStart   = "2026-01-15T00:00:00Z"
	requestTime   = "2026-01-14T00:00:00Z"
	maxReportRows = 100
	labelPatches  = "patches"
	captureWait   = 2 * time.Second
	capturePoll   = 10 * time.Millisecond
	// Every window starts hours from now, so no test ever meets a real window edge.
	windowLead = 2 * time.Hour
)

var eventGVR = schema.GroupVersionResource{Version: "v1", Resource: "events"}

// A series whose daily window opens windowLead from now, or one already open when open is set.
func series(phase string, open bool) plans.ProtectionPlan {
	from, to := windowLead, windowLead+time.Hour
	if open {
		from, to = -time.Hour, time.Hour
	}
	now := time.Now().UTC()
	return plans.ProtectionPlan{
		ID:       planID,
		Name:     "nightly freeze",
		Severity: plans.SeverityLow,
		Phase:    phase,
		Mode:     plans.ModeAudit,
		TimeMode: plans.TimeModeRepeating,
		Repeat: &plans.ProtectionPlanRepeat{
			Days:      slices.Clone(plans.RepeatDays),
			StartTime: now.Add(from).Format(clockLayout),
			EndTime:   now.Add(to).Format(clockLayout),
			TimeZone:  zoneUTC,
		},
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{planNS}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: templateID}},
	}
}

// Serves the plan on every read and records every patch; a set status fails that call instead.
type planStub struct {
	mu          sync.Mutex
	plan        plans.ProtectionPlan
	patches     []map[string]any
	getStatus   int
	patchStatus int
}

func (s *planStub) install(t *testing.T) {
	t.Helper()
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPatch {
			if s.getStatus != constants.DefaultInitValue {
				return answer(t, s.getStatus, nil), nil
			}
			return answer(t, http.StatusOK, s.plan), nil
		}
		if s.patchStatus != constants.DefaultInitValue {
			return answer(t, s.patchStatus, nil), nil
		}
		body := map[string]any{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode patch: %v", err)
		}
		s.mu.Lock()
		s.patches = append(s.patches, body)
		s.mu.Unlock()
		return answer(t, http.StatusOK, nil), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}

func (s *planStub) onlyPatch(t *testing.T) map[string]any {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	testutil.Equal(t, labelPatches, len(s.patches), constants.DefaultAddValue)
	return s.patches[constants.DefaultInitValue]
}

// Records whether the plan's policy was still in the cluster at each ledger write.
type ledgerStore struct {
	mu        sync.Mutex
	dyn       *dynamicfake.FakeDynamicClient
	liveAtPut []bool
	creates   []reportseps.CreatePlanReportRequest
}

func (s *ledgerStore) Create(req reportseps.CreatePlanReportRequest) (*reportseps.ReportMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates = append(s.creates, req)
	return &req.Meta, nil
}

func (s *ledgerStore) PutLedger(string, json.RawMessage) error {
	_, err := s.dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(planNS).Get(context.Background(), policyName, metav1.GetOptions{})
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liveAtPut = append(s.liveAtPut, err == nil)
	return nil
}

func (*ledgerStore) GetLedger(string) (json.RawMessage, error) { return nil, nil }

func (s *ledgerStore) firstCreate() (reportseps.CreatePlanReportRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.creates) == constants.DefaultInitValue {
		return reportseps.CreatePlanReportRequest{}, false
	}
	return s.creates[constants.DefaultInitValue], true
}

func (s *ledgerStore) awaitCreate(t *testing.T) reportseps.CreatePlanReportRequest {
	t.Helper()
	for deadline := time.Now().Add(captureWait); time.Now().Before(deadline); time.Sleep(capturePoll) {
		if report, ok := s.firstCreate(); ok {
			return report
		}
	}
	t.Fatal("no window report within the capture wait")
	return reportseps.CreatePlanReportRequest{}
}

func windowCluster() *dynamicfake.FakeDynamicClient {
	pol := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": protpolicies.KyvernoPolicyGVR.GroupVersion().String(),
		"kind":       "NamespacedValidatingPolicy",
		"metadata": map[string]any{
			"name":      policyName,
			"namespace": planNS,
			"labels":    map[string]any{dpolicies.LabelPlanID: planID, dpolicies.LabelManagedBy: dpolicies.ManagedByValue},
		},
	}}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		protpolicies.KyvernoPolicyGVR: "NamespacedValidatingPolicyList",
		eventGVR:                      "EventList",
	}, pol)
}

func windowService(dyn *dynamicfake.FakeDynamicClient, store *ledgerStore) *protection.Service {
	gen := reports.NewGenerator(store, dyn, nil, nil, nopLogger{}, maxReportRows, time.Minute)
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(protpolicies.KyvernoPolicyGVR.GroupVersion().WithKind(dpolicies.PolicyKind), meta.RESTScopeNamespace)
	applier := protpolicies.NewApplier(dyn, mapper)
	listNamespaces := func(context.Context) ([]string, error) { return []string{planNS}, nil }
	return protection.NewService(applier, nil, clients.NewProtectionPlanClient(), dyn, listNamespaces, gen, nopLogger{}, nil)
}

func windowOf(t *testing.T, patch map[string]any) map[string]any {
	t.Helper()
	window, ok := patch[protection.FieldWindow].(map[string]any)
	if !ok {
		t.Fatalf("patch window = %v, want an object", patch[protection.FieldWindow])
	}
	return window
}

// The next window starts a new ledger, so a closed window has to reach the ledger while its
// policies are still in the cluster, and it is one run with one report of its own.
func TestCloseWindowCheckpointsThenReportsTheWindow(t *testing.T) {
	plan := series(plans.PhaseActive, false)
	started := windowStart
	plan.StartedAt = &started
	plan.RenderedPolicies = []string{policyName}
	stub := &planStub{plan: plan}
	stub.install(t)
	dyn := windowCluster()
	store := &ledgerStore{dyn: dyn}

	if err := windowService(dyn, store).CloseWindow(context.Background(), &plan); err != nil {
		t.Fatalf("close window: %v", err)
	}

	patch := stub.onlyPatch(t)
	testutil.Equal(t, "phase", patch[protection.FieldPhase], any(plans.PhaseScheduled))
	testutil.Equal(t, "rendered policies cleared", len(patch[protection.FieldRenderedPolicies].([]any)), constants.DefaultInitValue)
	next, _ := plans.PlanWindow(&plan, time.Now())
	testutil.Equal(t, "next window", windowOf(t, patch)["startAt"], any(next.StartAt))
	_, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(planNS).Get(context.Background(), policyName, metav1.GetOptions{})
	testutil.Equal(t, "policy removed", err != nil, true)

	report := store.awaitCreate(t)
	run, err := reports.RunKey(&plan)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "report id", report.Meta.ID, run+"-"+reportseps.TriggerWindow)
	var doc reports.ReportDocument
	if err := json.Unmarshal([]byte(report.Files[reportseps.FormatJSON]), &doc); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	testutil.Equal(t, "report phase", doc.Cover.Phase, plans.PhaseScheduled)
	store.mu.Lock()
	defer store.mu.Unlock()
	testutil.Equal(t, "first ledger write saw the policy", len(store.liveAtPut) > constants.DefaultInitValue && store.liveAtPut[0], true)
}

// The controller's snapshot can be stale: a window open again, or a plan an edit already parked,
// is left alone.
func TestCloseWindowSkipsAWindowStillOpen(t *testing.T) {
	plan := series(plans.PhaseActive, true)
	stub := &planStub{plan: plan}
	stub.install(t)
	dyn := windowCluster()

	if err := windowService(dyn, &ledgerStore{dyn: dyn}).CloseWindow(context.Background(), &plan); err != nil {
		t.Fatalf("close window: %v", err)
	}
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultInitValue)
}

// Activation re-reads the plan: a window an edit moved away since the controller's pass is not opened.
func TestActivateSkipsAClosedWindow(t *testing.T) {
	plan := series(plans.PhaseScheduled, false)
	stub := &planStub{plan: plan}
	stub.install(t)
	dyn := windowCluster()

	if err := windowService(dyn, &ledgerStore{dyn: dyn}).Activate(context.Background(), &plan); err != nil {
		t.Fatalf("activate: %v", err)
	}
	testutil.Equal(t, labelPatches, len(stub.patches), constants.DefaultInitValue)
}

// One approval covers the series, so neither an approval nor a reactivation is ever refused as
// expired: between windows each lands scheduled, or parked when it needs approval, with the next window stamped.
func TestApproveAndReactivateStampTheNextWindow(t *testing.T) {
	testutil.ExcludedNamespaces(t)
	reactivate := func(svc *protection.Service, plan *plans.ProtectionPlan) error {
		_, err := svc.Reactivate(context.Background(), userID, plan.ID)
		return err
	}
	cases := []struct {
		name      string
		plan      func() plans.ProtectionPlan
		act       func(*protection.Service, *plans.ProtectionPlan) error
		wantPhase string
	}{
		{"approve", func() plans.ProtectionPlan {
			plan := series(plans.PhasePendingApproval, false)
			plan.ApprovalMode = plans.ApprovalModeRequired
			plan.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStatePending, RequestedBy: userID, RequestedAt: requestTime}
			return plan
		}, func(svc *protection.Service, plan *plans.ProtectionPlan) error {
			req := planseps.DecideProtectionPlanRequest{Decision: protection.DecisionApproved, RequestedAt: requestTime}
			_, err := svc.Decide(context.Background(), approverID, plan.ID, req)
			return err
		}, plans.PhaseScheduled},
		{"reactivate", func() plans.ProtectionPlan { return series(plans.PhaseCanceled, false) }, reactivate, plans.PhaseScheduled},
		{"reactivate needing approval", func() plans.ProtectionPlan {
			plan := series(plans.PhaseCanceled, false)
			plan.ApprovalMode = plans.ApprovalModeRequired
			return plan
		}, reactivate, plans.PhasePendingApproval},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := c.plan()
			stub := &planStub{plan: plan}
			stub.install(t)
			dyn := windowCluster()

			if err := c.act(windowService(dyn, &ledgerStore{dyn: dyn}), &plan); err != nil {
				t.Fatalf("%s: %v", c.name, err)
			}

			patch := stub.onlyPatch(t)
			testutil.Equal(t, "phase", patch[protection.FieldPhase], any(c.wantPhase))
			next, _ := plans.PlanWindow(&plan, time.Now())
			window := windowOf(t, patch)
			testutil.Equal(t, "window start", window["startAt"], any(next.StartAt))
			testutil.Equal(t, "window end", window["endAt"], any(next.EndAt))
			testutil.Equal(t, "nothing deployed", slices.ContainsFunc(dyn.Actions(), func(a k8stesting.Action) bool {
				return a.GetVerb() == "patch"
			}), false)
		})
	}
}

func TestRefreshWindowStampsOnlyAStaleWindow(t *testing.T) {
	current := series(plans.PhaseScheduled, false)
	current.Window, _ = plans.PlanWindow(&current, time.Now())
	noRepeat := series(plans.PhaseScheduled, false)
	noRepeat.Repeat = nil
	cases := []struct {
		name        string
		plan        plans.ProtectionPlan
		getStatus   int
		wantPatches int
		wantErr     bool
	}{
		{"stale window replaced", series(plans.PhaseScheduled, false), constants.DefaultInitValue, constants.DefaultAddValue, false},
		{"current window kept", current, constants.DefaultInitValue, constants.DefaultInitValue, false},
		{"no window to compute", noRepeat, constants.DefaultInitValue, constants.DefaultInitValue, false},
		{"plan unreadable", current, http.StatusNotFound, constants.DefaultInitValue, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &planStub{plan: c.plan, getStatus: c.getStatus}
			stub.install(t)
			dyn := windowCluster()

			err := windowService(dyn, &ledgerStore{dyn: dyn}).RefreshWindow(&c.plan)

			testutil.Equal(t, "error", err != nil, c.wantErr)
			testutil.Equal(t, labelPatches, len(stub.patches), c.wantPatches)
			if c.wantPatches != constants.DefaultInitValue {
				next, _ := plans.PlanWindow(&c.plan, time.Now())
				testutil.Equal(t, "window start", windowOf(t, stub.onlyPatch(t))["startAt"], any(next.StartAt))
			}
		})
	}
}

func TestCloseWindowReturnsExporterFailures(t *testing.T) {
	cases := []struct {
		name                   string
		getStatus, patchStatus int
	}{
		{"plan unreadable", http.StatusNotFound, constants.DefaultInitValue},
		{"patch refused", constants.DefaultInitValue, http.StatusConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := series(plans.PhaseActive, false)
			stub := &planStub{plan: plan, getStatus: c.getStatus, patchStatus: c.patchStatus}
			stub.install(t)
			dyn := windowCluster()

			err := windowService(dyn, &ledgerStore{dyn: dyn}).CloseWindow(context.Background(), &plan)

			testutil.Equal(t, "error", err != nil, true)
			_, getErr := dyn.Resource(protpolicies.KyvernoPolicyGVR).Namespace(planNS).Get(context.Background(), policyName, metav1.GetOptions{})
			testutil.Equal(t, "policy kept", getErr, nil)
		})
	}
}

// The window an activation opens is stamped with it, so the controller's next pass finds it current.
func TestActivateStampsTheOpenWindow(t *testing.T) {
	testutil.ExcludedNamespaces(t)
	plan := series(plans.PhaseScheduled, true)
	stub := &planStub{plan: plan}
	stub.install(t)
	dyn := windowCluster()
	// The fake has no server-side apply: the deploy is taken as applied.
	dyn.PrependReactor("patch", protpolicies.KyvernoPolicyGVR.Resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
		patch, ok := action.(k8stesting.PatchAction)
		return ok && patch.GetPatchType() == types.ApplyPatchType, nil, nil
	})

	if err := windowService(dyn, &ledgerStore{dyn: dyn}).Activate(context.Background(), &plan); err != nil {
		t.Fatalf("activate: %v", err)
	}

	patch := stub.onlyPatch(t)
	testutil.Equal(t, "phase", patch[protection.FieldPhase], any(plans.PhaseActive))
	open, _ := plans.PlanWindow(&plan, time.Now())
	testutil.Equal(t, "window start", windowOf(t, patch)["startAt"], any(open.StartAt))
}
