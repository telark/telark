package planctl

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	protectionctl "github.com/telark/telark/services/discovery/internal/controllers/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	tickEnv     = "PROTECTION_PLAN_TICK_INTERVAL_SEC"
	tickSeconds = "31"

	// RFC3339 drops sub-second precision, so an edge built this far out lands 1-2s ahead.
	boundaryLead = 2 * time.Second
	// A tick-bound controller would need the full 31s, so anything under this proves the
	// boundary timer fired and not the tick.
	maxLag = time.Second
	// Long enough for the edge plus scheduling slop, far short of one tick.
	settleWindow = 8 * time.Second
	idleWindow   = 2 * time.Second

	kindActivate  = "activate"
	kindTerminate = "terminate"
)

type event struct {
	kind   string
	planID string
	at     time.Time
}

type fakeService struct {
	mu          sync.Mutex
	plans       []plans.ProtectionPlan
	lists       int
	reconciles  int
	activateErr error
	events      chan event
}

func (f *fakeService) ListAllPlans() ([]plans.ProtectionPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists++
	return slices.Clone(f.plans), nil
}

func (f *fakeService) Activate(_ context.Context, plan *plans.ProtectionPlan) error {
	if f.activateErr != nil {
		return f.activateErr
	}
	f.setPhase(plan.ID, plans.PhaseActive)
	f.record(kindActivate, plan.ID)
	return nil
}

func (f *fakeService) Terminate(_ context.Context, plan *plans.ProtectionPlan) error {
	f.setPhase(plan.ID, plans.PhaseTerminated)
	f.record(kindTerminate, plan.ID)
	return nil
}

func (f *fakeService) ReconcileHealthForActive(_ context.Context, _ []plans.ProtectionPlan) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reconciles++
}

func (f *fakeService) setPhase(planID, phase string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.plans {
		if f.plans[i].ID == planID {
			f.plans[i].Phase = phase
		}
	}
}

func (f *fakeService) record(kind, planID string) {
	f.events <- event{kind: kind, planID: planID, at: time.Now().UTC()}
}

func (f *fakeService) counts() (lists, reconciles int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lists, f.reconciles
}

type silentLogger struct{}

func (silentLogger) Info(string)  {}
func (silentLogger) Error(string) {}

// edge returns the stored RFC3339 string and the instant the controller will parse back out of
// it, which truncation makes earlier than now+offset.
func edge(offset time.Duration) (string, time.Time) {
	raw := time.Now().UTC().Add(offset).Format(time.RFC3339)
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return raw, parsed
}

func plan(id, phase, startAt, endAt string) plans.ProtectionPlan {
	return plans.ProtectionPlan{
		ID:        id,
		Phase:     phase,
		TimeMode:  plans.TimeModeTimeRange,
		TimeRange: &plans.ProtectionPlanTimeRange{StartAt: startAt, EndAt: endAt},
	}
}

func start(t *testing.T, fake *fakeService) time.Time {
	t.Helper()
	t.Setenv(tickEnv, tickSeconds)
	fake.events = make(chan event, len(fake.plans)*2+1)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go protectionctl.NewController(fake, silentLogger{}).Run(ctx)
	return time.Now().UTC()
}

func await(t *testing.T, fake *fakeService, kind string) event {
	t.Helper()
	select {
	case ev := <-fake.events:
		testutil.Equal(t, "event kind", ev.kind, kind)
		return ev
	case <-time.After(settleWindow):
		t.Fatalf("no %s within %s: the controller waited for the tick", kind, settleWindow)
		return event{}
	}
}

func assertLag(t *testing.T, ev event, boundary time.Time) {
	t.Helper()
	if ev.at.Before(boundary) {
		t.Fatalf("%s fired at %s, before its boundary %s", ev.kind, ev.at, boundary)
	}
	if lag := ev.at.Sub(boundary); lag > maxLag {
		t.Fatalf("%s lagged its boundary by %s, want at most %s", ev.kind, lag, maxLag)
	}
}

// The measured defect: a plan declaring protection from startAt was not enforcing until the next
// periodic tick, leaving the first seconds of every window unprotected.
func TestActivationLandsOnStartAt(t *testing.T) {
	startAt, boundary := edge(boundaryLead)
	endAt, _ := edge(time.Hour)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-start", plans.PhaseScheduled, startAt, endAt)}}

	start(t, fake)
	assertLag(t, await(t, fake, kindActivate), boundary)
}

// The safe direction of the same defect: protection over-ran its window and the phase the UI
// reads stayed active for up to a tick past endAt.
func TestTerminationLandsOnEndAt(t *testing.T) {
	startAt, _ := edge(-time.Hour)
	endAt, boundary := edge(boundaryLead)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-end", plans.PhaseActive, startAt, endAt)}}

	start(t, fake)
	assertLag(t, await(t, fake, kindTerminate), boundary)
}

// A new leader (or a restarted process) inherits plans whose boundary already passed and must
// close the gap immediately rather than at its first tick.
func TestOverdueBoundaryFiresOnLeadershipStart(t *testing.T) {
	startAt, _ := edge(-time.Hour)
	endAt, _ := edge(time.Hour)
	pastEnd, _ := edge(-time.Minute)
	fake := &fakeService{plans: []plans.ProtectionPlan{
		plan("plan-overdue", plans.PhaseScheduled, startAt, endAt),
		plan("plan-overdue-pending", plans.PhasePendingApproval, startAt, pastEnd),
	}}

	startedAt := start(t, fake)
	assertLag(t, await(t, fake, kindActivate), startedAt)
	assertLag(t, await(t, fake, kindTerminate), startedAt)
}

// Approval is what a pending plan waits on, not its startAt: a window that opens before the
// decision lands must not start enforcing on its own.
func TestPendingIsNotActivatedAtStartAt(t *testing.T) {
	startAt, _ := edge(-time.Hour)
	endAt, _ := edge(time.Hour)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-pending", plans.PhasePendingApproval, startAt, endAt)}}

	start(t, fake)
	time.Sleep(idleWindow)

	testutil.Equal(t, "events for a pending plan", len(fake.events), constants.DefaultInitValue)
}

// A pending plan whose window closes is over: it is terminated on its endAt like an active one,
// so a late approval finds nothing to start.
func TestPendingTerminatesAtEndAt(t *testing.T) {
	startAt, _ := edge(-time.Hour)
	endAt, boundary := edge(boundaryLead)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-pending-end", plans.PhasePendingApproval, startAt, endAt)}}

	start(t, fake)
	assertLag(t, await(t, fake, kindTerminate), boundary)
	testutil.Equal(t, "further events", len(fake.events), constants.DefaultInitValue)
}

// The plan the controller just activated still carries phase=scheduled in memory, so its endAt
// has to be armed from the window itself or a short plan over-runs by a full tick.
func TestShortWindowTerminatesOnTime(t *testing.T) {
	startAt, _ := edge(boundaryLead)
	endAt, boundary := edge(boundaryLead * 2)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-short", plans.PhaseScheduled, startAt, endAt)}}

	start(t, fake)
	await(t, fake, kindActivate)
	assertLag(t, await(t, fake, kindTerminate), boundary)
}

// A transition that fails leaves its edge in the past. Re-arming on past edges would busy-loop
// the exporter; the retry belongs on the tick.
func TestFailedTransitionDoesNotSpin(t *testing.T) {
	startAt, _ := edge(-time.Hour)
	endAt, _ := edge(time.Hour)
	fake := &fakeService{
		plans:       []plans.ProtectionPlan{plan("plan-stuck", plans.PhaseScheduled, startAt, endAt)},
		activateErr: errors.New("exporter unavailable"),
	}

	start(t, fake)
	time.Sleep(idleWindow)

	lists, _ := fake.counts()
	testutil.Equal(t, "list calls while stuck", lists, constants.DefaultAddValue)
}

// Boundary wakes must not drag the health reconcile along: one cluster-wide policy LIST plus a
// PATCH per active plan per boundary would multiply cluster load by the number of distinct edges.
func TestHealthReconcileStaysOnTheTick(t *testing.T) {
	startAt, _ := edge(boundaryLead)
	endAt, _ := edge(time.Hour)
	fake := &fakeService{plans: []plans.ProtectionPlan{plan("plan-health", plans.PhaseScheduled, startAt, endAt)}}

	start(t, fake)
	await(t, fake, kindActivate)

	_, reconciles := fake.counts()
	testutil.Equal(t, "health reconciles before the first tick", reconciles, constants.DefaultInitValue)
}
