package planctl

import (
	"slices"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	zoneUTC     = "UTC"
	clockLayout = "15:04"
	// Hours away from every minute boundary, so a test never waits for a real window edge: those
	// are covered by the window math's own tests.
	windowMargin = time.Hour
)

// Every day, so the window is the same whichever weekday the test runs on.
func repeating(id, phase string, from, to time.Duration) plans.ProtectionPlan {
	now := time.Now().UTC()
	return plans.ProtectionPlan{
		ID:       id,
		Phase:    phase,
		TimeMode: plans.TimeModeRepeating,
		Repeat: &plans.ProtectionPlanRepeat{
			Days:      slices.Clone(plans.RepeatDays),
			StartTime: now.Add(from).Format(clockLayout),
			EndTime:   now.Add(to).Format(clockLayout),
			TimeZone:  zoneUTC,
		},
	}
}

func withCurrentWindow(plan plans.ProtectionPlan) plans.ProtectionPlan {
	plan.Window, _ = plans.PlanWindow(&plan, time.Now())
	return plan
}

// A new leader inherits a series whose window opened while no pass ran: it starts enforcing at
// once, not at its first tick.
func TestOpenWindowActivatesAtStart(t *testing.T) {
	fake := &fakeService{plans: []plans.ProtectionPlan{
		repeating("series-open", plans.PhaseScheduled, -windowMargin, windowMargin),
	}}

	startedAt := start(t, fake)
	assertLag(t, await(t, fake, kindActivate), startedAt)
}

// Time closes a window but never terminates the series: the plan goes back to scheduled.
func TestEndedWindowClosesAtStart(t *testing.T) {
	fake := &fakeService{plans: []plans.ProtectionPlan{
		repeating("series-ended", plans.PhaseActive, 2*windowMargin, 3*windowMargin),
	}}

	startedAt := start(t, fake)
	assertLag(t, await(t, fake, kindClose), startedAt)
}

// Approval is what a pending series waits on: an open window neither activates nor ends it.
func TestPendingSeriesIsNeverActivatedOrTerminated(t *testing.T) {
	fake := &fakeService{plans: []plans.ProtectionPlan{
		withCurrentWindow(repeating("series-pending", plans.PhasePendingApproval, -windowMargin, windowMargin)),
	}}

	start(t, fake)
	time.Sleep(idleWindow)

	testutil.Equal(t, "events for a pending series", len(fake.events), constants.DefaultInitValue)
}

// A window that passed while the plan waited (for approval, or for a leader) leaves a stale stored
// window, which the pass replaces with the one in force.
func TestStaleWindowIsRefreshed(t *testing.T) {
	fake := &fakeService{plans: []plans.ProtectionPlan{
		repeating("series-stale", plans.PhaseScheduled, 2*windowMargin, 3*windowMargin),
	}}

	start(t, fake)
	await(t, fake, kindRefresh)
}
