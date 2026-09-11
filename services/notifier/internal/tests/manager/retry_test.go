package manager

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telark/notifier/internal/subscribers/manager"
	"github.com/telark/notifier/internal/tests/testutil"
)

const (
	testRetryInterval = 5 * time.Millisecond
	testCancelWait    = 50 * time.Millisecond
	failTwice         = 2
)

var errStartFailed = errors.New("nats unavailable")

// flakyStart fails the first n calls, then succeeds — the shape of a NATS
// outage that heals while the service keeps running.
func flakyStart(n int32, calls *atomic.Int32) func() error {
	return func() error {
		if calls.Add(1) <= n {
			return errStartFailed
		}
		return nil
	}
}

// A NATS outage at boot must heal on its own: RetryStart keeps calling Start
// until it connects, so the operator never has to restart the service. This is
// the reported bug — Start was called exactly once and the failure was only
// logged.
func TestRetryStartRetriesUntilSuccess(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(failTwice, &calls), testRetryInterval, nil); err != nil {
		t.Fatalf("RetryStart: %v", err)
	}
	testutil.Equal(t, "start attempts", calls.Load(), failTwice+1)
}

// A Start that succeeds first time must not be retried.
func TestRetryStartStopsAfterFirstSuccess(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(0, &calls), testRetryInterval, nil); err != nil {
		t.Fatalf("RetryStart: %v", err)
	}
	testutil.Equal(t, "start attempts", calls.Load(), 1)
}

// Shutdown must win over the retry loop: a permanently failing Start has to
// stop when the context is cancelled, not spin forever and block exit.
func TestRetryStartStopsOnContextCancel(t *testing.T) {
	var calls atomic.Int32
	alwaysFails := func() error { calls.Add(1); return errStartFailed }

	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- manager.RetryStart(ctx, alwaysFails, testRetryInterval, nil) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RetryStart err = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(testCancelWait * 10):
		t.Fatal("RetryStart did not stop after context cancellation")
	}
	if calls.Load() == 0 {
		t.Fatal("RetryStart never called start")
	}
}

// recordingLogger captures what the retry loop reports to the operator.
type recordingLogger struct {
	errs  atomic.Int32
	warns atomic.Int32
}

func (l *recordingLogger) Error(string) { l.errs.Add(1) }
func (l *recordingLogger) Warn(string)  { l.warns.Add(1) }

// Every failed attempt must tell the operator what broke and that a retry is
// coming — otherwise a stuck notifier looks identical to a healthy one.
func TestRetryStartLogsEachFailedAttempt(t *testing.T) {
	var calls atomic.Int32
	lg := &recordingLogger{}
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(failTwice, &calls), testRetryInterval, lg); err != nil {
		t.Fatalf("RetryStart: %v", err)
	}
	testutil.Equal(t, "error logs", lg.errs.Load(), failTwice)
	testutil.Equal(t, "retry warnings", lg.warns.Load(), failTwice)
}

// A non-positive interval must fall back to the configured default rather than
// busy-looping on a zero-length timer.
func TestRetryStartDefaultsNonPositiveInterval(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(0, &calls), 0, nil); err != nil {
		t.Fatalf("RetryStart: %v", err)
	}
	testutil.Equal(t, "start attempts", calls.Load(), 1)
}

// An already-cancelled context must not trigger a connection attempt at all.
func TestRetryStartHonoursCancelledContext(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := manager.RetryStart(ctx, flakyStart(0, &calls), testRetryInterval, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("RetryStart err = %v, want context.Canceled", err)
	}
	testutil.Equal(t, "start attempts", calls.Load(), 0)
}
