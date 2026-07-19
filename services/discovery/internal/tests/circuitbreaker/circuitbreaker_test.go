package circuitbreaker

import (
	"errors"
	"testing"
	"time"

	"github.com/telark/discovery/internal/circuitbreaker"
)

// New applies safe defaults for any unset config field so a zero-value config
// still yields a usable breaker.
func TestNewAppliesDefaults(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{})
	if cb == nil {
		t.Fatal("New returned nil")
	}
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatalf("default breaker rejected a success: %v", err)
	}
}

// The breaker opens after the failure threshold, short-circuits calls while
// open, then recovers through half-open on the next success after the timeout.
func TestBreakerTripsAndRecovers(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 1,
		Timeout:          10 * time.Millisecond,
	})
	boom := errors.New("boom")

	// Trip it: two failures reach the threshold.
	_ = cb.Execute(func() error { return boom })
	_ = cb.Execute(func() error { return boom })

	// Open: the function is not even called.
	called := false
	if err := cb.Execute(func() error { called = true; return nil }); err == nil || called {
		t.Fatalf("open circuit should short-circuit (err=%v, called=%v)", err, called)
	}

	// After the timeout, half-open lets a probe through; a success closes it.
	time.Sleep(15 * time.Millisecond)
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatalf("half-open probe rejected: %v", err)
	}
	if err := cb.Execute(func() error { return nil }); err != nil {
		t.Fatalf("closed breaker rejected a success: %v", err)
	}
}
