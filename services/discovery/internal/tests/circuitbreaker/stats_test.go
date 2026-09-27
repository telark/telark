package circuitbreaker

import (
	"errors"
	"testing"

	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/tests/testutil"
)

// A named breaker reports its name, tracks failure/success counts through Stats,
// and Reset returns it to the closed state.
func TestBreakerStatsNameReset(t *testing.T) {
	cb := circuitbreaker.New(circuitbreaker.Config{
		Name:             "redis",
		FailureThreshold: 3,
		SuccessThreshold: 1,
	})
	testutil.Equal(t, "name", cb.GetName(), "redis")
	testutil.Equal(t, "starts closed", cb.GetState(), circuitbreaker.StateClosed)

	boom := errors.New("boom")
	_ = cb.Execute(func() error { return boom })
	if cb.GetStats().FailureCount == constants.DefaultInitValue {
		t.Fatal("stats did not record the failure")
	}

	cb.Reset()
	testutil.Equal(t, "reset state", cb.GetState(), circuitbreaker.StateClosed)
	testutil.Equal(t, "reset failures", cb.GetStats().FailureCount, constants.DefaultInitValue)
}
