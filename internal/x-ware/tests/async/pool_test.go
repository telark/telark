package async

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telark/x-ware/async"
)

type recorder struct {
	mu       sync.Mutex
	warnings []string
}

func (r *recorder) Warn(message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.warnings = append(r.warnings, message)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.warnings)
}

func newPool(size int, log async.Logger) *async.Pool {
	return async.New(async.Config{
		Size:         size,
		TaskTimeout:  time.Second,
		DrainTimeout: time.Second,
		Logger:       log,
	})
}

func TestDispatchRunsWork(t *testing.T) {
	pool := newPool(2, &recorder{})
	var ran atomic.Bool

	pool.Dispatch(func(context.Context) { ran.Store(true) })
	pool.Drain()

	if !ran.Load() {
		t.Error("dispatched work never ran")
	}
}

// The task's context carries the deadline; work that honors it stops on time.
func TestDispatchGivesTaskADeadline(t *testing.T) {
	pool := async.New(async.Config{
		Size:         1,
		TaskTimeout:  20 * time.Millisecond,
		DrainTimeout: time.Second,
		Logger:       &recorder{},
	})
	var deadlineSet, expired atomic.Bool

	pool.Dispatch(func(ctx context.Context) {
		_, ok := ctx.Deadline()
		deadlineSet.Store(ok)
		<-ctx.Done()
		expired.Store(true)
	})
	pool.Drain()

	if !deadlineSet.Load() {
		t.Error("task context carried no deadline")
	}
	if !expired.Load() {
		t.Error("task context never expired")
	}
}

// A full pool drops rather than queueing without bound, or a burst would spawn
// unlimited goroutines.
func TestDispatchDropsWhenFull(t *testing.T) {
	log := &recorder{}
	pool := newPool(1, log)
	release := make(chan struct{})
	var ran atomic.Int32

	pool.Dispatch(func(context.Context) {
		<-release
		ran.Add(1)
	})
	for range 5 {
		pool.Dispatch(func(context.Context) { ran.Add(1) })
	}

	close(release)
	pool.Drain()

	if got := ran.Load(); got != 1 {
		t.Errorf("ran %d tasks, want 1: the pool did not bound concurrency", got)
	}
	if log.count() == 0 {
		t.Error("dropped tasks passed silently")
	}
}

// Shutdown must not hang on work that ignores its context.
func TestDrainGivesUpOnStuckWork(t *testing.T) {
	log := &recorder{}
	pool := async.New(async.Config{
		Size:         1,
		TaskTimeout:  time.Minute,
		DrainTimeout: 20 * time.Millisecond,
		Logger:       log,
	})
	release := make(chan struct{})
	defer close(release)

	pool.Dispatch(func(context.Context) { <-release })

	done := make(chan struct{})
	go func() {
		pool.Drain()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Drain hung on a task that ignores its context")
	}

	if log.count() == 0 {
		t.Error("drain timeout passed silently")
	}
}

func TestDispatchOnNilPoolIsSafe(t *testing.T) {
	var pool *async.Pool

	pool.Dispatch(func(context.Context) { t.Error("nil pool ran work") })
	pool.Drain()
}

func TestNoLoggerIsSafe(t *testing.T) {
	pool := newPool(1, nil)
	release := make(chan struct{})

	pool.Dispatch(func(context.Context) { <-release })
	pool.Dispatch(func(context.Context) {})

	close(release)
	pool.Drain()
}
