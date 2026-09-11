package circuitbreaker

import (
	"errors"
	"fmt"
	"time"

	"github.com/telark/discovery/internal/constants"
)

var (
	ErrOpen       = errors.New(string(constants.ErrCircuitBreakerOpenSentinel))
	ErrNotCounted = errors.New(string(constants.ErrCircuitBreakerNotCounted))
)

func (e openError) Error() string {
	return fmt.Sprintf(string(constants.ErrCircuitBreakerOpen), e.name)
}

func (openError) Unwrap() error { return ErrOpen }

func (e notCountedError) Error() string { return e.err.Error() }

func (e notCountedError) Unwrap() []error { return []error{e.err, ErrNotCounted} }

func IsOpen(err error) bool { return errors.Is(err, ErrOpen) }

func NotCounted(err error) error {
	if err == nil {
		return nil
	}
	return notCountedError{err: err}
}

func New(cfg Config) *CircuitBreaker {
	if cfg.FailureThreshold <= constants.DefaultInitValue {
		cfg.FailureThreshold = constants.CircuitBreakerDefaultFailureThreshold
	}
	if cfg.SuccessThreshold <= constants.DefaultInitValue {
		cfg.SuccessThreshold = constants.CircuitBreakerDefaultSuccessThreshold
	}
	if cfg.Timeout <= constants.ZeroDuration {
		cfg.Timeout = constants.CircuitBreakerDefaultTimeout
	}

	return &CircuitBreaker{
		name:             cfg.Name,
		failureThreshold: cfg.FailureThreshold,
		successThreshold: cfg.SuccessThreshold,
		timeout:          cfg.Timeout,
		state:            StateClosed,
		lastStateChange:  time.Now(),
		onStateChange:    cfg.OnStateChange,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	allowed, isProbe := cb.canAttempt()
	if !allowed {
		return openError{name: cb.name}
	}
	if isProbe {
		defer cb.releaseProbe()
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) canAttempt() (allowed, isProbe bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.transitionTo(StateHalfOpen)
	}

	switch cb.state {
	case StateClosed:
		return true, false
	case StateHalfOpen:
		if cb.halfOpenProbes >= constants.CircuitBreakerHalfOpenMaxProbes {
			return false, false
		}
		cb.halfOpenProbes++
		return true, true
	default:
		return false, false
	}
}

// The slot is released even when the probe reopened the circuit; transitionTo
// zeroes the counter, so the guard keeps it from going negative.
func (cb *CircuitBreaker) releaseProbe() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.halfOpenProbes > constants.DefaultInitValue {
		cb.halfOpenProbes--
	}
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if errors.Is(err, ErrNotCounted) {
		return
	}

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

func (cb *CircuitBreaker) onFailure() {
	cb.failureCount++
	cb.successCount = constants.DefaultInitValue
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.transitionTo(StateOpen)
		}
	case StateHalfOpen:
		cb.transitionTo(StateOpen)
	default:
	}
}

func (cb *CircuitBreaker) onSuccess() {
	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.transitionTo(StateClosed)
		}
	case StateClosed:
		cb.failureCount = constants.DefaultInitValue
	default:
	}
}

func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.halfOpenProbes = constants.DefaultInitValue

	if newState == StateClosed {
		cb.failureCount = constants.DefaultInitValue
		cb.successCount = constants.DefaultInitValue
	}

	if cb.onStateChange != nil {
		cb.onStateChange(oldState, newState)
	}
}

func (cb *CircuitBreaker) GetState() State {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

func (cb *CircuitBreaker) GetName() string {
	return cb.name
}

func (cb *CircuitBreaker) GetStats() Stats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return Stats{
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastStateChange: cb.lastStateChange,
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.failureCount = constants.DefaultInitValue
	cb.successCount = constants.DefaultInitValue
	cb.halfOpenProbes = constants.DefaultInitValue
	cb.lastStateChange = time.Now()

	if cb.onStateChange != nil && oldState != StateClosed {
		cb.onStateChange(oldState, StateClosed)
	}
}
