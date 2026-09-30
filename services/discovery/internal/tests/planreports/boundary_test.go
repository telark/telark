package planreports

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/rest/response"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	planStarted      = "plan-started"
	planEnded        = "plan-ended"
	planScheduled    = "plan-scheduled"
	planTerminated   = "plan-terminated"
	planUnstarted    = "plan-unstarted"
	exporterDownBody = "exporter down"
	eligiblePlans    = 3
)

// countingStore records every GetLedger call: one per Checkpoint the service actually made.
type countingStore struct {
	*fakeStore
	mu     sync.Mutex
	gets   []string
	getErr error
}

func (s *countingStore) GetLedger(planID string) (json.RawMessage, error) {
	s.mu.Lock()
	s.gets = append(s.gets, planID)
	s.mu.Unlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.fakeStore.GetLedger(planID)
}

func (s *countingStore) ledgerGets() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.gets)
}

func newService(store *countingStore) (*protection.Service, *memLogger) {
	logger := &memLogger{}
	gen := reports.NewGenerator(store, fakeDyn(), resolveOnlyA, nil, logger, maxRows, time.Minute)
	return protection.NewService(nil, resolveOnlyA, nil, fakeDyn(), nil, gen, logger, nil), logger
}

func planIn(id, phase string) plans.ProtectionPlan {
	p := activePlan()
	p.ID = id
	p.Phase = phase
	return *p
}

func TestCheckpointReportsSkipsInactiveAndTerminated(t *testing.T) {
	store := &countingStore{fakeStore: &fakeStore{}}
	svc, _ := newService(store)
	ended := planIn(planEnded, plans.PhaseActive)
	ended.TerminatedAt = ptr(startedAt)
	unstarted := planIn(planUnstarted, plans.PhaseActive)
	unstarted.StartedAt = nil
	list := []plans.ProtectionPlan{
		planIn(planStarted, plans.PhaseActive),
		ended,
		planIn(planScheduled, plans.PhaseScheduled),
		planIn(planTerminated, plans.PhaseTerminated),
		unstarted,
	}

	svc.CheckpointReports(context.Background(), list)

	testutil.Equal(t, "ledger gets", strings.Join(store.ledgerGets(), comma), planStarted)
	puts, _, _ := store.snapshot()
	testutil.Equal(t, namePuts, len(puts), one)
}

func TestCheckpointReportsBreaksOnFirstExporterFailure(t *testing.T) {
	store := &countingStore{fakeStore: &fakeStore{}}
	store.getErr = clients.ClassifyReportResponse(&response.GenericResponse{
		Status: http.StatusInternalServerError, Message: exporterDownBody,
	})
	svc, logger := newService(store)
	list := make([]plans.ProtectionPlan, zero, eligiblePlans)
	for i := range eligiblePlans {
		list = append(list, planIn(fmt.Sprintf("%s-%d", planStarted, i), plans.PhaseActive))
	}

	svc.CheckpointReports(context.Background(), list)

	testutil.Equal(t, "ledger gets", len(store.ledgerGets()), one)
	testutil.Equal(t, "failure logged", logger.has(exporterDownBody), true)
}

// GenerateReport gates on reports.ManualAllowed; the post-end phases it must let through are
// pinned here (draft/scheduled/unstarted rejections live in TestManualAllowedRejects...).
func TestGenerateReportPhaseGate(t *testing.T) {
	for _, phase := range []string{plans.PhaseFailed, plans.PhaseCanceled, plans.PhaseTerminated} {
		plan := activePlan()
		plan.Phase = phase
		plan.TerminatedAt = ptr(startedAt)
		testutil.Equal(t, phase+" allowed", reports.ManualAllowed(plan), nil)
	}
}
