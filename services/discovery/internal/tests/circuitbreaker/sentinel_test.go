package circuitbreaker

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/tests/testutil"
)

const farPastThresholdCalls = 50

var errBoom = errors.New("boom")

func trippedBreaker(t *testing.T, timeout time.Duration) *circuitbreaker.CircuitBreaker {
	t.Helper()
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name:             "exporter",
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          timeout,
	})
	_ = cb.Execute(func() error { return errBoom })
	_ = cb.Execute(func() error { return errBoom })
	testutil.Equal(t, "state after threshold", cb.GetState(), circuitbreaker.StateOpen)
	return cb
}

// Once the failure threshold trips the breaker, callers can detect the rejection
// with IsOpen instead of string-matching the error.
func TestIsOpenDetectsRejection(t *testing.T) {
	cb := trippedBreaker(t, time.Minute)

	err := cb.Execute(func() error { return nil })
	if !circuitbreaker.IsOpen(err) {
		t.Fatalf("IsOpen(%v) = false, want true", err)
	}
	if circuitbreaker.IsOpen(errBoom) {
		t.Fatal("IsOpen matched an unrelated error")
	}
}

// Errors wrapped with NotCounted reach the caller unchanged and never move the
// breaker towards open, however often they occur.
func TestNotCountedErrorsNeverTrip(t *testing.T) {
	cases := []struct {
		name  string
		calls int
	}{
		{name: "threshold", calls: constants.TwoValue},
		{name: "far past threshold", calls: farPastThresholdCalls},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cb := circuitbreaker.New(circuitbreaker.Config{
				Name:             "exporter",
				FailureThreshold: 2,
				SuccessThreshold: 1,
			})

			for range tc.calls {
				err := cb.Execute(func() error { return circuitbreaker.NotCounted(errBoom) })
				if !errors.Is(err, errBoom) || !errors.Is(err, circuitbreaker.ErrNotCounted) {
					t.Fatalf("Execute returned %v, want both errBoom and ErrNotCounted", err)
				}
			}

			testutil.Equal(t, "state", cb.GetState(), circuitbreaker.StateClosed)
			testutil.Equal(t, "failures", cb.GetStats().FailureCount, 0)
		})
	}
}

func TestNotCountedNilStaysNil(t *testing.T) {
	if err := circuitbreaker.NotCounted(nil); err != nil {
		t.Fatalf("NotCounted(nil) = %v, want nil", err)
	}
}

// Half-open admits a single probe; every concurrent caller is rejected as open
// rather than piling onto a recovering dependency.
func TestHalfOpenAdmitsOneProbe(t *testing.T) {
	cb := trippedBreaker(t, 10*time.Millisecond)
	time.Sleep(15 * time.Millisecond)

	var (
		release  = make(chan struct{})
		started  sync.WaitGroup
		rejected error
	)
	started.Add(constants.DefaultAddValue)

	var probe sync.WaitGroup
	probe.Add(constants.DefaultAddValue)
	go func() {
		defer probe.Done()
		_ = cb.Execute(func() error {
			started.Done()
			<-release
			return nil
		})
	}()

	started.Wait()
	before := cb.GetStats().FailureCount
	rejected = cb.Execute(func() error { return nil })
	close(release)
	probe.Wait()

	if !circuitbreaker.IsOpen(rejected) {
		t.Fatalf("concurrent half-open call = %v, want open rejection", rejected)
	}
	testutil.Equal(t, "no failure recorded for rejection", cb.GetStats().FailureCount, before)
}

// A failed probe reopens the circuit and still frees its slot, so the next
// probe after the timeout is admitted instead of deadlocking in half-open.
func TestFailedProbeReleasesSlot(t *testing.T) {
	cb := trippedBreaker(t, 10*time.Millisecond)

	time.Sleep(15 * time.Millisecond)
	if err := cb.Execute(func() error { return errBoom }); !errors.Is(err, errBoom) {
		t.Fatalf("first probe = %v, want errBoom", err)
	}
	testutil.Equal(t, "reopened", cb.GetState(), circuitbreaker.StateOpen)

	time.Sleep(15 * time.Millisecond)
	probed := false
	if err := cb.Execute(func() error { probed = true; return nil }); err != nil || !probed {
		t.Fatalf("second probe blocked (err=%v, probed=%v)", err, probed)
	}
}

// Half-open closes only after successThreshold consecutive successful probes.
func TestHalfOpenClosesAfterSuccessThreshold(t *testing.T) {
	cb := trippedBreaker(t, 10*time.Millisecond)
	time.Sleep(15 * time.Millisecond)

	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatalf("first probe rejected: %v", err)
	}
	testutil.Equal(t, "still half-open", cb.GetState(), circuitbreaker.StateHalfOpen)

	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatalf("second probe rejected: %v", err)
	}
	testutil.Equal(t, "closed", cb.GetState(), circuitbreaker.StateClosed)
}
