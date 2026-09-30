package planupdate

import (
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/update"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	pastStart   = "2026-01-01T00:00:00Z"
	pastEnd     = "2026-01-02T00:00:00Z"
	futureStart = "2026-03-01T00:00:00Z"
	futureEnd   = "2026-03-02T00:00:00Z"
)

var editNow = time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)

func window(start, end string) *planseps.TimeRangeRequest {
	return &planseps.TimeRangeRequest{StartAt: start, EndAt: end}
}

// The regression: editing the window never recomputed the phase, so a scheduled plan made
// permanent never deployed and an active plan given a future start kept enforcing.
func TestTargetPhaseFollowsEditedWindow(t *testing.T) {
	cases := []struct {
		name     string
		phase    string
		timeMode string
		tr       *planseps.TimeRangeRequest
		want     string
	}{
		{"scheduled to permanent", plans.PhaseScheduled, plans.TimeModePermanent, nil, plans.PhaseActive},
		{"scheduled to open window", plans.PhaseScheduled, plans.TimeModeTimeRange, window(pastStart, futureEnd), plans.PhaseActive},
		{"scheduled stays future", plans.PhaseScheduled, plans.TimeModeTimeRange, window(futureStart, futureEnd), plans.PhaseScheduled},
		{"active to future window", plans.PhaseActive, plans.TimeModeTimeRange, window(futureStart, futureEnd), plans.PhaseScheduled},
		{"active stays permanent", plans.PhaseActive, plans.TimeModePermanent, nil, plans.PhaseActive},
		{"active elapsed window left to controller", plans.PhaseActive, plans.TimeModeTimeRange, window(pastStart, pastEnd), plans.PhaseActive},
		{"pending untouched", plans.PhasePendingApproval, plans.TimeModePermanent, nil, plans.PhasePendingApproval},
		{"canceled untouched", plans.PhaseCanceled, plans.TimeModeTimeRange, window(pastStart, futureEnd), plans.PhaseCanceled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := &plans.ProtectionPlan{Phase: c.phase, TimeMode: plans.TimeModeTimeRange}
			req := &planseps.PrepareProtectionPlanRequest{TimeMode: c.timeMode, TimeRange: c.tr}
			testutil.Equal(t, "phase", update.TargetPhase(plan, req, editNow), c.want)
		})
	}
}
