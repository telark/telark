package rollbackctl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/handlers/rollback"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/client-go/util/workqueue"
)

const (
	waitTimeout  = 2 * time.Second
	pollInterval = 10 * time.Millisecond
)

// Every handler parks until released, so N arrivals are only reachable when N
// workers drain the queue side by side; a single-worker loop times out on the
// second arrival. The loop must also outlive every worker and return only
// after ctx is canceled.
func TestRunWorkersDrainsKeysConcurrently(t *testing.T) {
	const workers = 4
	const keys = workers * 2
	queue := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]())
	for i := range keys {
		queue.Add(fmt.Sprintf("ns/app-%d", i))
	}

	arrived := make(chan struct{}, keys)
	release := make(chan struct{})
	var processed atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		rollback.RunWorkers(ctx, queue, workers, func(context.Context, string) error {
			arrived <- struct{}{}
			<-release
			processed.Add(constants.DefaultAddValue)
			return nil
		})
		close(done)
	}()

	for range workers {
		select {
		case <-arrived:
		case <-time.After(waitTimeout):
			t.Fatalf("fewer than %d keys in flight at once", workers)
		}
	}
	select {
	case <-done:
		t.Fatal("RunWorkers returned before ctx was canceled")
	default:
	}
	close(release)

	deadline := time.Now().Add(waitTimeout)
	for processed.Load() < keys && time.Now().Before(deadline) {
		time.Sleep(pollInterval)
	}
	testutil.Equal(t, "processed keys", processed.Load(), int32(keys))

	cancel()
	select {
	case <-done:
	case <-time.After(waitTimeout):
		t.Fatal("RunWorkers did not return after cancel")
	}
}
