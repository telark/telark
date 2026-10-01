package manager

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telark/telark/services/notifier/internal/constants"
	"github.com/telark/telark/services/notifier/internal/subscribers/manager"
	"github.com/telark/telark/services/notifier/internal/tests/testutil"
)

const (
	testRetryInterval = 5 * time.Millisecond
	zeroInterval      = time.Duration(0)
	testCancelWait    = 50 * time.Millisecond
	failTwice         = 2
	neverFail         = 0
	oneAttempt        = 1
	retryStartFailFmt = "RetryStart: %v"
	startAttempts     = "start attempts"
)

var errStartFailed = errors.New("nats unavailable")

// flakyStart fails the first n calls, then succeeds — the shape of a NATS
// outage that heals while the service keeps running.
func flakyStart(n int32, calls *atomic.Int32) func() error {
	return func() error {
		if calls.Add(constants.DefaultAdd) <= n {
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
		t.Fatalf(retryStartFailFmt, err)
	}
	testutil.Equal(t, startAttempts, calls.Load(), failTwice+oneAttempt)
}

// A Start that succeeds first time must not be retried.
func TestRetryStartStopsAfterFirstSuccess(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(neverFail, &calls), testRetryInterval, nil); err != nil {
		t.Fatalf(retryStartFailFmt, err)
	}
	testutil.Equal(t, startAttempts, calls.Load(), oneAttempt)
}

// Shutdown must win over the retry loop: a permanently failing Start has to
// stop when the context is canceled, not spin forever and block exit.
func TestRetryStartStopsOnContextCancel(t *testing.T) {
	var calls atomic.Int32
	alwaysFails := func() error { calls.Add(constants.DefaultAdd); return errStartFailed }

	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	done := make(chan error, oneAttempt)
	go func() { done <- manager.RetryStart(ctx, alwaysFails, testRetryInterval, nil) }()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RetryStart err = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(testCancelWait * 10):
		t.Fatal("RetryStart did not stop after context cancellation")
	}
	if calls.Load() == neverFail {
		t.Fatal("RetryStart never called start")
	}
}

// recordingLogger captures what the retry loop reports to the operator.
type recordingLogger struct {
	errs  atomic.Int32
	warns atomic.Int32
}

func (l *recordingLogger) Error(string) { l.errs.Add(constants.DefaultAdd) }
func (l *recordingLogger) Warn(string)  { l.warns.Add(constants.DefaultAdd) }

// Every failed attempt must tell the operator what broke and that a retry is
// coming — otherwise a stuck notifier looks identical to a healthy one.
func TestRetryStartLogsEachFailedAttempt(t *testing.T) {
	var calls atomic.Int32
	lg := &recordingLogger{}
	ctx, cancel := context.WithTimeout(context.Background(), testCancelWait)
	defer cancel()

	if err := manager.RetryStart(ctx, flakyStart(failTwice, &calls), testRetryInterval, lg); err != nil {
		t.Fatalf(retryStartFailFmt, err)
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

	if err := manager.RetryStart(ctx, flakyStart(neverFail, &calls), zeroInterval, nil); err != nil {
		t.Fatalf(retryStartFailFmt, err)
	}
	testutil.Equal(t, startAttempts, calls.Load(), oneAttempt)
}

// An already-canceled context must not trigger a connection attempt at all.
func TestRetryStartHonorsCanceledContext(t *testing.T) {
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := manager.RetryStart(ctx, flakyStart(neverFail, &calls), testRetryInterval, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("RetryStart err = %v, want context.Canceled", err)
	}
	testutil.Equal(t, startAttempts, calls.Load(), neverFail)
}
