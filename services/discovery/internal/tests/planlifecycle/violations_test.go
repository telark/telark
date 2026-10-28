package planlifecycle

import (
	"context"
	"net/http"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const unreadableStart = "not-a-time"

// The live window reads the cluster's Events; window=run reads the run, which a plan that never
// started does not have yet and a plan with an unreadable start cannot name.
func TestListViolationsByWindow(t *testing.T) {
	started, badStart := windowStart, unreadableStart
	live := series(plans.PhaseActive, true)
	live.StartedAt = &started
	live.RenderedPolicies = []string{policyName}
	garbled := series(plans.PhaseTerminated, false)
	garbled.StartedAt = &badStart
	cases := []struct {
		name      string
		plan      plans.ProtectionPlan
		getStatus int
		window    string
		wantErr   bool
	}{
		{"live window", live, constants.DefaultInitValue, constants.EmptyString, false},
		{"run of a plan that never started", series(plans.PhaseScheduled, false), constants.DefaultInitValue, violations.WindowRun, false},
		{"run with an unreadable start", garbled, constants.DefaultInitValue, violations.WindowRun, true},
		{"plan unreadable", live, http.StatusNotFound, violations.WindowRun, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			stub := &planStub{plan: c.plan, getStatus: c.getStatus}
			stub.install(t)
			dyn := windowCluster()

			got, err := windowService(dyn, &ledgerStore{dyn: dyn}).ListViolations(context.Background(), planID, violations.Query{Window: c.window})

			testutil.Equal(t, "error", err != nil, c.wantErr)
			if !c.wantErr {
				testutil.Equal(t, "rows", got.Total, constants.DefaultInitValue)
			}
		})
	}
}
