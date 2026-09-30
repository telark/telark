package planctl

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	protectionctl "github.com/telark/telark/services/discovery/internal/controllers/plans/protection"
)

const (
	checkpointInterval = 20 * time.Millisecond
	checkpointWindow   = 70 * time.Millisecond
	minCheckpointRuns  = 3
)

type fakeCheckpointService struct {
	mu    sync.Mutex
	calls []time.Time
}

func (*fakeCheckpointService) ListAllPlans() ([]plans.ProtectionPlan, error) {
	return nil, nil
}

func (f *fakeCheckpointService) CheckpointReports(context.Context, []plans.ProtectionPlan) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, time.Now())
}

func (f *fakeCheckpointService) snapshot() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]time.Time, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestCheckpointRunsOnStartThenOnInterval(t *testing.T) {
	svc := &fakeCheckpointService{}
	ctrl := protectionctl.NewCheckpointController(svc, &silentLogger{}, checkpointInterval)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	start := time.Now()
	go ctrl.Run(ctx)
	time.Sleep(checkpointWindow)
	calls := svc.snapshot()
	if len(calls) < minCheckpointRuns {
		t.Fatalf("expected at least %d checkpoint runs, got %d", minCheckpointRuns, len(calls))
	}
	if !calls[0].Before(start.Add(checkpointInterval)) {
		t.Fatalf("first checkpoint ran at +%s, expected before the first tick (%s)",
			calls[0].Sub(start), checkpointInterval)
	}
}

func TestCheckpointStopsOnContextCancel(t *testing.T) {
	svc := &fakeCheckpointService{}
	ctrl := protectionctl.NewCheckpointController(svc, &silentLogger{}, checkpointInterval)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	time.Sleep(checkpointInterval)
	cancel()
	select {
	case <-done:
	case <-time.After(checkpointWindow):
		t.Fatal("Run did not return after context cancel")
	}
	before := len(svc.snapshot())
	time.Sleep(checkpointWindow)
	if after := len(svc.snapshot()); after != before {
		t.Fatalf("checkpoint kept running after cancel: %d -> %d", before, after)
	}
}
