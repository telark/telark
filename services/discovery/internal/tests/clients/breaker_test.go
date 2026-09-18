package clients

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/telark/discovery/internal/circuitbreaker"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/rest/response"
)

const failuresPastThreshold = constants.CircuitBreakerRestFailureThreshold * 4

func resetExporter(t *testing.T) {
	t.Helper()
	circuitbreaker.GetManager().Reset(circuitbreaker.DependencyExporter)
	t.Cleanup(func() {
		circuitbreaker.GetManager().Reset(circuitbreaker.DependencyExporter)
	})
}

func statusResponse(status int) *response.GenericResponse {
	return &response.GenericResponse{Status: status, Message: "x"}
}

// A routine 404 repeated well past the failure threshold must never open the
// circuit — a breaker that trips on them would self-inflict an outage.
func TestRoutineStatusesDoNotOpenCircuit(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusBadRequest, http.StatusConflict} {
		resetExporter(t)
		calls := constants.DefaultInitValue
		for range failuresPastThreshold {
			err := clients.GuardStatusError(constants.ErrRestCallFailed, func() *response.GenericResponse {
				calls++
				return statusResponse(status)
			})
			if err == nil {
				t.Fatalf("status %d: caller lost the error", status)
			}
		}
		if calls != failuresPastThreshold {
			t.Fatalf("status %d: calls = %d, want %d", status, calls, failuresPastThreshold)
		}
		if state := circuitbreaker.GetManager().GetState(circuitbreaker.DependencyExporter); state != circuitbreaker.StateClosed {
			t.Fatalf("status %d opened the circuit (state=%v)", status, state)
		}
	}
}

// 500 is what rest-pkg synthesizes for transport and connectivity-gate
// failures, so it is the one status that must trip the breaker.
func TestServerErrorOpensCircuitAndSkipsIO(t *testing.T) {
	resetExporter(t)
	for range constants.CircuitBreakerRestFailureThreshold {
		_ = clients.GuardStatusError(constants.ErrRestCallFailed, func() *response.GenericResponse {
			return statusResponse(http.StatusInternalServerError)
		})
	}
	if state := circuitbreaker.GetManager().GetState(circuitbreaker.DependencyExporter); state != circuitbreaker.StateOpen {
		t.Fatalf("state = %v, want Open", state)
	}

	calls := constants.DefaultInitValue
	err := clients.GuardStatusError(constants.ErrRestCallFailed, func() *response.GenericResponse {
		calls++
		return statusResponse(http.StatusOK)
	})
	if !circuitbreaker.IsOpen(err) {
		t.Fatalf("err = %v, want an open-circuit error", err)
	}
	if calls != constants.DefaultInitValue {
		t.Fatalf("open circuit still performed %d call(s)", calls)
	}
}

// The gate error carries no sentinel, so only its rest-pkg prefix identifies it.
func TestConnectivityGateErrorCounts(t *testing.T) {
	gateErr := errors.New(clients.ConnectivityNotReadyPrefix() + "exporter")
	if clients.ClassifyForBreaker(gateErr) == nil || circuitbreaker.NotCounted(gateErr) == nil {
		t.Fatal("classification dropped the error")
	}
	if errors.Is(clients.ClassifyForBreaker(gateErr), circuitbreaker.ErrNotCounted) {
		t.Fatal("connectivity gate failure must count towards the breaker")
	}
	opaque := errors.New("unexpected status: 404: nope")
	if !errors.Is(clients.ClassifyForBreaker(opaque), circuitbreaker.ErrNotCounted) {
		t.Fatal("an opaque non-connectivity error must not count")
	}
}

// An open circuit makes every retry a guaranteed instant failure, so the loop
// must abort on the first attempt instead of sleeping through all of them.
func TestCreateSnapshotStopsRetryingWhenOpen(t *testing.T) {
	t.Setenv(constants.EnvSnapshotWriteMaxAttempts, "3")
	t.Setenv(constants.EnvSnapshotWriteRetryIntervalSec, "2")
	resetExporter(t)
	for range constants.CircuitBreakerRestFailureThreshold {
		_ = clients.GuardStatusError(constants.ErrRestCallFailed, func() *response.GenericResponse {
			return statusResponse(http.StatusInternalServerError)
		})
	}

	start := time.Now()
	path, err := clients.NewSnapshotClient().CreateSnapshotAndReturnPath("id", "scope", "ns", 1, nil)
	elapsed := time.Since(start)

	if !circuitbreaker.IsOpen(err) {
		t.Fatalf("err = %v, want an open-circuit error", err)
	}
	if path != constants.EmptyString {
		t.Fatalf("path = %q, want empty", path)
	}
	if elapsed > time.Second {
		t.Fatalf("retry loop burned %v, want a single immediate attempt", elapsed)
	}
}
