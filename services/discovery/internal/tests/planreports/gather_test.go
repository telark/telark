package planreports

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	reportseps "github.com/telark/rest/endpoints/reports"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	planPolicy    = "telark-pp-a1-b2-c3"
	planNS        = "pp-ns"
	startedAt     = "2026-09-20T10:00:00Z"
	runKey        = "20260920T100000Z"
	actor         = "user-42"
	appA          = "app-a"
	appB          = "app-b"
	maxRows       = 5000
	opPut         = "put"
	opCreate      = "create"
	minuteStep    = time.Minute
	blockedDetail = "PersistentVolumeClaim pp-ns/claim: [block-pvc] fail (blocked); pvc mutation blocked"
	keyAPIVersion = "apiVersion"
	keyKind       = "kind"
	keyName       = "name"
	keyNamespace  = "namespace"
	comma         = ","
	uidOne        = "u1"
	nameCreates   = "creates"
	nameDocRows   = "doc rows"
	namePuts      = "puts"
	nameTruncated = "truncated"
	zero          = 0
	one           = 1
	two           = 2
	afterOne      = minuteStep
	afterTwo      = 2 * minuteStep
	afterThree    = 3 * minuteStep
	afterFive     = 5 * minuteStep
	afterTen      = 10 * minuteStep
	afterThirty   = 30 * minuteStep
	afterForty    = 40 * minuteStep
	threeHours    = 3 * time.Hour
)

var eventGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}

type fakeStore struct {
	mu          sync.Mutex
	stored      []byte
	lastPlan    string
	puts        [][]byte
	creates     []reportseps.CreatePlanReportRequest
	ops         []string
	putErr      error
	createPanic bool
	createDelay time.Duration
	createSpans [][2]time.Time
	onPut       func()
	release     chan struct{}
	started     chan struct{}
}

func (s *fakeStore) Create(req reportseps.CreatePlanReportRequest) (*reportseps.ReportMeta, error) {
	if s.createPanic {
		panic("render exploded")
	}
	if s.started != nil {
		s.started <- struct{}{}
	}
	if s.release != nil {
		<-s.release
	}
	begin := time.Now()
	time.Sleep(s.createDelay)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creates = append(s.creates, req)
	s.ops = append(s.ops, opCreate)
	s.createSpans = append(s.createSpans, [2]time.Time{begin, time.Now()})
	meta := req.Meta
	return &meta, nil
}

func (s *fakeStore) PutLedger(planID string, ledger json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPlan = planID
	if s.onPut != nil {
		s.onPut()
	}
	if s.putErr != nil {
		return s.putErr
	}
	s.puts = append(s.puts, slices.Clone(ledger))
	s.ops = append(s.ops, opPut)
	return nil
}

func (s *fakeStore) GetLedger(planID string) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastPlan = planID
	return s.stored, nil
}

func (s *fakeStore) snapshot() (puts [][]byte, creates []reportseps.CreatePlanReportRequest, ops []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.puts), slices.Clone(s.creates), slices.Clone(s.ops)
}

type memLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *memLogger) Info(msg string)  { l.add(msg) }
func (l *memLogger) Error(msg string) { l.add(msg) }

func (l *memLogger) add(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, msg)
}

func (l *memLogger) has(sub string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.ContainsFunc(l.lines, func(line string) bool { return strings.Contains(line, sub) })
}

func resolveOnlyA(context.Context, []string) (map[string]policies.ResolvedApp, []string, error) {
	return map[string]policies.ResolvedApp{appA: {Namespaces: []string{planNS}}}, []string{appB}, nil
}

func newGen(store *fakeStore, dyn dynamic.Interface, rdb *redis.Client) (*reports.Generator, *memLogger) {
	logger := &memLogger{}
	return reports.NewGenerator(store, dyn, resolveOnlyA, rdb, logger, maxRows, time.Minute), logger
}

func ptr(s string) *string { return &s }

func activePlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:               planID,
		Name:             "guard storage",
		Severity:         plans.SeverityHigh,
		Priority:         one,
		Scope:            plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{planNS}},
		Policies:         []plans.ProtectionPlanPolicy{{TemplateID: "block-pvc", Params: map[string]any{"kinds": "pvc"}}},
		Mode:             plans.ModeEnforce,
		TimeMode:         plans.TimeModePermanent,
		Phase:            plans.PhaseActive,
		RenderedPolicies: []string{planPolicy},
		CreatedAt:        startedAt,
		CreatedBy:        actor,
		LastUpdatedAt:    startedAt,
		LastUpdatedBy:    actor,
		StartedAt:        ptr(startedAt),
		StartedBy:        ptr(actor),
		ParticipantsIDs:  []string{actor, "user-7"},
		Health:           plans.HealthHealthy,
		HealthCheckedAt:  ptr(startedAt),
		HealthDetail: []plans.ProtectionPlanHealthDetail{
			{PolicyName: planPolicy, Namespace: planNS, Present: true, Ready: true, FailureAction: "Enforce"},
		},
	}
}

func stamp(base string, offset time.Duration) string {
	t, err := time.Parse(time.RFC3339, base)
	if err != nil {
		panic(err)
	}
	return t.Add(offset).UTC().Format(time.RFC3339)
}

func violationEvent(name, uid, policy, eventTime string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion: "v1",
		keyKind:       "Event",
		"metadata":    map[string]any{keyName: name, keyNamespace: planNS, "uid": uid},
		"reason":      "PolicyViolation",
		"type":        "Warning",
		"message":     blockedDetail,
		"eventTime":   eventTime,
		"involvedObject": map[string]any{
			keyAPIVersion: "kyverno.io/v1",
			keyKind:       "Policy",
			keyName:       policy,
			keyNamespace:  planNS,
		},
		"related": map[string]any{
			keyAPIVersion: "v1",
			keyKind:       "PersistentVolumeClaim",
			keyName:       "claim",
			keyNamespace:  planNS,
		},
	}}
}

func fakeDyn(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{eventGVR: "EventList"},
		objs...,
	)
}

func row(uid, ts, message string) planseps.ProtectionPlanViolation {
	return planseps.ProtectionPlanViolation{
		Policy:    planPolicy,
		Rule:      "block-pvc",
		Namespace: planNS,
		Resource:  planseps.ProtectionPlanResource{Kind: "PersistentVolumeClaim", Name: "claim", Namespace: planNS},
		Result:    "fail",
		Message:   message,
		Timestamp: ts,
		EventUID:  uid,
	}
}

func ledgerBytes(t *testing.T, ledger reports.PlanReportLedger) []byte {
	t.Helper()
	raw, err := json.Marshal(ledger)
	if err != nil {
		t.Fatalf("marshal ledger: %v", err)
	}
	return raw
}

func storedLedger(t *testing.T, run string, rows ...planseps.ProtectionPlanViolation) []byte {
	t.Helper()
	return ledgerBytes(t, reports.PlanReportLedger{PlanID: planID, Run: run, RenderedPolicies: []string{planPolicy}, Violations: rows})
}

func decodeLedger(t *testing.T, raw []byte) reports.PlanReportLedger {
	t.Helper()
	var ledger reports.PlanReportLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		t.Fatalf("decode ledger: %v", err)
	}
	return ledger
}

func decodeDoc(t *testing.T, req reportseps.CreatePlanReportRequest) reports.ReportDocument {
	t.Helper()
	var doc reports.ReportDocument
	if err := json.Unmarshal([]byte(req.Files[reportseps.FormatJSON]), &doc); err != nil {
		t.Fatalf("decode document: %v", err)
	}
	return doc
}

func gather(t *testing.T, plan *plans.ProtectionPlan, ledger *reports.PlanReportLedger, at string) *reports.ReportDocument {
	t.Helper()
	gen, _ := newGen(&fakeStore{}, fakeDyn(), nil)
	generatedAt, err := time.Parse(time.RFC3339, at)
	if err != nil {
		t.Fatalf("parse generatedAt: %v", err)
	}
	doc, err := gen.Gather(context.Background(), plan, ledger, actor, reportseps.TriggerManual, generatedAt)
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	return doc
}

func TestGatherActivePlanPopulatesAllSections(t *testing.T) {
	ledger := &reports.PlanReportLedger{
		PlanID:           planID,
		Run:              runKey,
		RenderedPolicies: []string{planPolicy},
		Checkpoints:      []reports.LedgerCheckpoint{{At: stamp(startedAt, afterOne), Health: plans.HealthHealthy, ViolationsSeen: two}},
		Violations:       []planseps.ProtectionPlanViolation{row(uidOne, stamp(startedAt, afterOne), "m1"), row("u2", startedAt, "m2")},
	}
	generatedAt := stamp(startedAt, afterTwo)

	doc := gather(t, activePlan(), ledger, generatedAt)

	testutil.Equal(t, "cover name", doc.Cover.Name, "guard storage")
	testutil.Equal(t, "cover mode", doc.Cover.Mode, reports.ModeLabelEnforce)
	testutil.Equal(t, "report id", doc.Cover.ReportID, runKey+"-manual-20260920T100200Z")
	testutil.Equal(t, "checkpoints", doc.Coverage.Checkpoints, one)
	testutil.Equal(t, "policies live", doc.Coverage.PoliciesLiveAtCapture, true)
	testutil.Equal(t, "gaps", len(doc.Coverage.Gaps), zero)
	testutil.Equal(t, "row cap", doc.Coverage.RowCap, maxRows)
	testutil.Equal(t, "timeline started", doc.Timeline.StartedAt, startedAt)
	testutil.Equal(t, "scope ns", strings.Join(doc.Scope.Namespaces, comma), planNS)
	testutil.Equal(t, "policy blocks", len(doc.Policies), one)
	testutil.Equal(t, "policy rendered", doc.Policies[zero].RenderedName, planPolicy)
	testutil.Equal(t, "policy ready", doc.Policies[zero].Ready, true)
	testutil.Equal(t, "health", doc.Health.Value, plans.HealthHealthy)
	testutil.Equal(t, "health timeline", len(doc.Health.Timeline), one)
	testutil.Equal(t, "total", doc.Decisions.Aggregates.Total, two)
	testutil.Equal(t, "by result", doc.Decisions.Aggregates.ByResult[zero].Count, two)
	testutil.Equal(t, "appendix plan", doc.Appendix.PlanID, planID)
	testutil.Equal(t, "appendix generator", doc.Appendix.Generator, reports.GeneratorName)
	testutil.Equal(t, "provenance rows", len(doc.Coverage.Provenance) > 0, true)
}

func TestGatherTerminatedPlanUsesLedgerPoliciesAndMarksRemoved(t *testing.T) {
	plan := activePlan()
	plan.Phase = plans.PhaseTerminated
	plan.RenderedPolicies = nil
	plan.Health = plans.HealthUnknown
	plan.TerminatedAt = ptr(stamp(startedAt, afterThirty))
	ledger := &reports.PlanReportLedger{PlanID: planID, Run: runKey, RenderedPolicies: []string{planPolicy}}

	doc := gather(t, plan, ledger, stamp(startedAt, afterForty))

	testutil.Equal(t, "policies live", doc.Coverage.PoliciesLiveAtCapture, false)
	testutil.Equal(t, "rendered from ledger", strings.Join(doc.Appendix.RenderedPolicies, comma), planPolicy)
	testutil.Equal(t, "policy block rendered", doc.Policies[zero].RenderedName, planPolicy)
	testutil.Equal(t, "post-end note", slices.Contains(doc.Coverage.Notes, reports.SentencePostEnd), true)
	testutil.Equal(t, "run end", doc.Cover.RunEnd, *plan.TerminatedAt)
}

func TestGatherFlagsTrailingGapAfterLastCheckpoint(t *testing.T) {
	checkpointAt := stamp(startedAt, afterTen)
	ledger := &reports.PlanReportLedger{
		PlanID:      planID,
		Run:         runKey,
		Checkpoints: []reports.LedgerCheckpoint{{At: checkpointAt, Health: plans.HealthHealthy}},
	}
	end := stamp(startedAt, threeHours)

	doc := gather(t, activePlan(), ledger, end)

	testutil.Equal(t, "gap count", len(doc.Coverage.Gaps), one)
	testutil.Equal(t, "gap from", doc.Coverage.Gaps[zero].From, checkpointAt)
	testutil.Equal(t, "gap to", doc.Coverage.Gaps[zero].To, end)
	testutil.Equal(t, "first checkpoint", doc.Coverage.FirstCheckpoint, checkpointAt)
	testutil.Equal(t, "last checkpoint", doc.Coverage.LastCheckpoint, checkpointAt)
}

func TestGatherDegradesOnMissingApplication(t *testing.T) {
	plan := activePlan()
	plan.Scope = plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{appA, appB}}

	doc := gather(t, plan, &reports.PlanReportLedger{PlanID: planID, Run: runKey}, stamp(startedAt, afterOne))

	testutil.Equal(t, "namespaces", strings.Join(doc.Scope.Namespaces, comma), planNS)
	testutil.Equal(t, "unresolved", strings.Join(doc.Coverage.UnresolvedApplications, comma), appB)
	testutil.Equal(t, "scope apps", strings.Join(doc.Scope.ApplicationIDs, comma), appA+comma+appB)
}

func TestGatherTimelineHoldsRawIDs(t *testing.T) {
	plan := activePlan()
	plan.TerminatedAt = ptr(stamp(startedAt, afterOne))
	plan.TerminatedBy = ptr("user-9")

	doc := gather(t, plan, &reports.PlanReportLedger{PlanID: planID, Run: runKey}, stamp(startedAt, afterTwo))

	testutil.Equal(t, "created by", doc.Timeline.CreatedBy, actor)
	testutil.Equal(t, "started by", doc.Timeline.StartedBy, actor)
	testutil.Equal(t, "terminated by", doc.Timeline.TerminatedBy, "user-9")
	testutil.Equal(t, "participants", strings.Join(doc.Timeline.ParticipantsIDs, comma), actor+comma+"user-7")
	testutil.Equal(t, "generated by", doc.Cover.GeneratedBy, actor)
}

var errPut = errors.New("ledger store down")
