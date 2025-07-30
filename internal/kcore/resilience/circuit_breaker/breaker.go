package circuit_breaker

import (
	"sync"
	"time"

	"github.com/plsyro/kcore-pkg/constants"
)

type (
	CircuitBreakerState int
	CircuitBreaker      struct {
		state           CircuitBreakerState
		failureCount    int
		lastFailureTime time.Time
		mu              sync.RWMutex
		maxFailures     int
		timeout         time.Duration
		resetTimeout    time.Duration
	}
)

func NewCircuitBreaker(maxFailures int, timeout, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:        CLOSED,
		maxFailures:  maxFailures,
		timeout:      timeout,
		resetTimeout: resetTimeout,
	}
}

func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.canExecute() {
		return ErrCircuitBreakerOpen
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CLOSED:
		return true
	case OPEN:
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = HALF_OPEN
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case HALF_OPEN:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.maxFailures {
			cb.state = OPEN
		}
	} else {
		cb.failureCount = 0
		cb.state = CLOSED
	}
}

func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CLOSED
	cb.failureCount = 0
}

var ErrCircuitBreakerOpen = &CircuitBreakerError{}

type CircuitBreakerError struct{}

func (e *CircuitBreakerError) Error() string {
	return constants.CIRCUIT_BREAKER_IS_OPEN
}
