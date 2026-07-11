package circuitbreaker

import (
	"fmt"
	"time"

	"github.com/telark/discovery/constants"
)

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
	if !cb.canAttempt() {
		return fmt.Errorf(string(constants.ErrCircuitBreakerOpen), cb.name)
	}

	err := fn()
	cb.recordResult(err)
	return err
}

func (cb *CircuitBreaker) canAttempt() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if cb.state == StateClosed || cb.state == StateHalfOpen {
		return true
	}

	if cb.state == StateOpen && time.Since(cb.lastFailureTime) > cb.timeout {
		cb.transitionTo(StateHalfOpen)
		return true
	}

	return false
}

func (cb *CircuitBreaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

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
	cb.lastStateChange = time.Now()

	if cb.onStateChange != nil && oldState != StateClosed {
		cb.onStateChange(oldState, StateClosed)
	}
}
