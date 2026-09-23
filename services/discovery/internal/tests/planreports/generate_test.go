package planreports

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	reportseps "github.com/telark/rest/endpoints/reports"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	mergedRows   = 3
	blockFor     = 300 * time.Millisecond
	busyDeadline = 100 * time.Millisecond
	waitFor      = 2 * time.Second
	reasonEnded  = "Plan window ended."
)

// Two rows already in the ledger plus one live Event inside the run window.
func mergedFixture(t *testing.T) *fakeStore {
	t.Helper()
	return &fakeStore{stored: storedLedger(t, runKey,
		row(uidOne, stamp(startedAt, afterOne), "one"),
		row("u2", stamp(startedAt, afterTwo), "two"),
	)}
}

func liveDyn() *dynamicfake.FakeDynamicClient {
	return fakeDyn(violationEvent("evt-3", "u3", planPolicy, stamp(startedAt, afterThree)))
}

func generate(t *testing.T, store *fakeStore, plan *plans.ProtectionPlan, trigger string) *reportseps.ReportMeta {
	t.Helper()
	gen, _ := newGen(store, liveDyn(), nil)
	meta, err := gen.Generate(context.Background(), plan, actor, trigger)
	if err != nil {
		t.Fatalf("generate %s: %v", trigger, err)
	}
	return meta
}

func TestGenerateMergesLiveThenRendersFromMergedLedger(t *testing.T) {
	store := mergedFixture(t)

	meta := generate(t, store, activePlan(), reportseps.TriggerEnd)

	puts, creates, ops := store.snapshot()
	testutil.Equal(t, namePuts, len(puts), one)
	testutil.Equal(t, nameCreates, len(creates), one)
	testutil.Equal(t, "order", strings.Join(ops, ","), opPut+","+opCreate)
	ledger := decodeLedger(t, puts[zero])
	testutil.Equal(t, "ledger rows", len(ledger.Violations), mergedRows)
	testutil.Equal(t, "ledger checkpoints", len(ledger.Checkpoints), one)
	testutil.Equal(t, "checkpoint seen", ledger.Checkpoints[zero].ViolationsSeen, one)
	testutil.Equal(t, "ledger policies", strings.Join(ledger.RenderedPolicies, ","), planPolicy)
	testutil.Equal(t, nameDocRows, len(decodeDoc(t, creates[zero]).Decisions.Rows), mergedRows)
	testutil.Equal(t, "meta id", meta.ID, runKey+"-"+reportseps.TriggerEnd)
	testutil.Equal(t, "meta total", meta.ViolationsTotal, mergedRows)
	testutil.Equal(t, "meta trigger", creates[zero].Meta.Trigger, reportseps.TriggerEnd)
	testutil.Equal(t, "formats", len(creates[zero].Files), len(reportseps.Formats))
}

func TestGenerateBoundaryPersistsLedgerAndManualNeverWrites(t *testing.T) {
	store := mergedFixture(t)

	meta := generate(t, store, activePlan(), reportseps.TriggerManual)

	puts, creates, _ := store.snapshot()
	testutil.Equal(t, namePuts, len(puts), zero)
	testutil.Equal(t, nameCreates, len(creates), one)
	testutil.Equal(t, nameDocRows, len(decodeDoc(t, creates[zero]).Decisions.Rows), mergedRows)
	testutil.Equal(t, "manual id", strings.HasPrefix(meta.ID, runKey+"-"+reportseps.TriggerManual+"-"), true)
}

func TestGenerateRendersWhenLedgerWriteFails(t *testing.T) {
	store := mergedFixture(t)
	store.putErr = errPut

	generate(t, store, activePlan(), reportseps.TriggerCancel)

	_, creates, _ := store.snapshot()
	testutil.Equal(t, nameCreates, len(creates), one)
	testutil.Equal(t, nameDocRows, len(decodeDoc(t, creates[zero]).Decisions.Rows), mergedRows)
}

func TestGenerateManualAfterEndMergesLiveRowsUpToTerminatedAt(t *testing.T) {
	terminatedAt := time.Now().UTC().Add(-afterTen).Format(time.RFC3339)
	plan := activePlan()
	plan.Phase = plans.PhaseTerminated
	plan.RenderedPolicies = nil
	plan.StartedAt = ptr(stamp(terminatedAt, -time.Hour))
	plan.TerminatedAt = ptr(terminatedAt)
	run, err := reports.RunKey(plan)
	if err != nil {
		t.Fatalf("run key: %v", err)
	}
	store := &fakeStore{stored: storedLedger(t, run, row(uidOne, stamp(terminatedAt, -afterThirty), "ledger"))}
	dyn := fakeDyn(
		violationEvent("evt-in", "u-in", planPolicy, stamp(terminatedAt, -afterOne)),
		violationEvent("evt-late", "u-late", planPolicy, stamp(terminatedAt, afterOne)),
	)
	gen, _ := newGen(store, dyn, nil)

	if _, err := gen.Generate(context.Background(), plan, actor, reportseps.TriggerManual); err != nil {
		t.Fatalf("generate: %v", err)
	}

	puts, creates, _ := store.snapshot()
	testutil.Equal(t, namePuts, len(puts), zero)
	doc := decodeDoc(t, creates[0])
	testutil.Equal(t, "rows", len(doc.Decisions.Rows), two)
	testutil.Equal(t, "late excluded", strings.Contains(creates[zero].Files[reportseps.FormatCSV], "u-late"), false)
	testutil.Equal(t, "post-end sentence", strings.Contains(creates[zero].Files[reportseps.FormatMarkdown], reports.SentencePostEnd), true)
}

func TestGeneratePanicInRenderBecomesErrorNotCrash(t *testing.T) {
	store := mergedFixture(t)
	store.createPanic = true
	gen, logger := newGen(store, liveDyn(), nil)

	_, err := gen.Generate(context.Background(), activePlan(), actor, reportseps.TriggerEnd)

	if err == nil || !strings.Contains(err.Error(), strings.TrimSuffix(string(constants.ErrReportPanic), "%v")) {
		t.Fatalf("expected a panic error, got %v", err)
	}
	testutil.Equal(t, "panic logged", logger.has("render exploded"), true)
}

func waitDone(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(waitFor):
		t.Fatal("capture did not finish")
	}
}

func TestCaptureAsyncSkipsUnstartedPlans(t *testing.T) {
	store := &fakeStore{}
	gen, _ := newGen(store, fakeDyn(), nil)
	scheduled := activePlan()
	scheduled.Phase = plans.PhaseScheduled
	unstarted := activePlan()
	unstarted.StartedAt = nil

	for _, plan := range []*plans.ProtectionPlan{scheduled, unstarted} {
		waitDone(t, gen.CaptureAsync(plan, startedAt, actor, reasonEnded, reportseps.TriggerEnd))
	}

	puts, creates, _ := store.snapshot()
	testutil.Equal(t, namePuts, len(puts), zero)
	testutil.Equal(t, nameCreates, len(creates), zero)
}

func TestCaptureAsyncSerializesAndNeverTakesManualSemaphore(t *testing.T) {
	store := mergedFixture(t)
	store.createDelay = blockFor
	gen, logger := newGen(store, liveDyn(), nil)
	endedAt := stamp(startedAt, time.Hour)

	first := gen.CaptureAsync(activePlan(), endedAt, actor, reasonEnded, reportseps.TriggerEnd)
	second := gen.CaptureAsync(activePlan(), endedAt, actor, reasonEnded, reportseps.TriggerCancel)
	_, err := gen.Generate(context.Background(), activePlan(), actor, reportseps.TriggerManual)
	if err != nil {
		t.Fatalf("manual generate during captures: %v", err)
	}
	waitDone(t, first)
	waitDone(t, second)

	_, creates, _ := store.snapshot()
	testutil.Equal(t, nameCreates, len(creates), mergedRows)
	var spans [][2]time.Time
	store.mu.Lock()
	for i, req := range creates {
		if req.Meta.Trigger != reportseps.TriggerManual {
			spans = append(spans, store.createSpans[i])
		}
	}
	store.mu.Unlock()
	testutil.Equal(t, "capture spans", len(spans), two)
	testutil.Equal(t, "serialized", !spans[one][zero].Before(spans[zero][one]), true)
	testutil.Equal(t, "captured logged", logger.has(reportseps.TriggerCancel), true)
	manualFirst := creates[zero].Meta.Trigger == reportseps.TriggerManual
	testutil.Equal(t, "phase stamped", manualFirst || decodeDoc(t, creates[zero]).Cover.Phase != plans.PhaseActive, true)
}

func TestGenerateManualBusyReturnsErrRenderBusy(t *testing.T) {
	store := mergedFixture(t)
	store.started = make(chan struct{}, reports.MaxConcurrentRenders)
	store.release = make(chan struct{})
	gen, _ := newGen(store, liveDyn(), nil)
	results := make(chan error, reports.MaxConcurrentRenders)
	for range reports.MaxConcurrentRenders {
		go func() {
			_, err := gen.Generate(context.Background(), activePlan(), actor, reportseps.TriggerManual)
			results <- err
		}()
	}
	for range reports.MaxConcurrentRenders {
		<-store.started
	}

	begin := time.Now()
	_, err := gen.Generate(context.Background(), activePlan(), actor, reportseps.TriggerManual)
	elapsed := time.Since(begin)
	close(store.release)
	for range reports.MaxConcurrentRenders {
		if err := <-results; err != nil {
			t.Fatalf("held render: %v", err)
		}
	}

	testutil.Equal(t, "busy", errors.Is(err, reports.ErrRenderBusy), true)
	testutil.Equal(t, "fast", elapsed < busyDeadline, true)
}

func TestManualAllowedRejectsDraftScheduledAndUnstarted(t *testing.T) {
	for _, phase := range []string{plans.PhaseDraft, plans.PhaseScheduled} {
		plan := activePlan()
		plan.Phase = phase
		err := reports.ManualAllowed(plan)
		testutil.Equal(t, phase+" validation", validation.IsValidation(err), true)
	}
	unstarted := activePlan()
	unstarted.StartedAt = nil
	err := reports.ManualAllowed(unstarted)
	testutil.Equal(t, "unstarted validation", validation.IsValidation(err), true)
	testutil.Equal(t, "unstarted text", strings.Contains(err.Error(), "has not started"), true)
	testutil.Equal(t, "active allowed", reports.ManualAllowed(activePlan()), nil)
}
