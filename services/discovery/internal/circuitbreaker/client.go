package circuitbreaker

import (
	"fmt"
	"sync"

	"github.com/telark/discovery/internal/constants"
)

var (
	instance *Manager
	once     sync.Once

	restDependencies = []DependencyType{
		DependencyExporter, DependencyAuth, DependencyNotifier, DependencyEnrichment,
	}
)

func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			breakers: make(map[DependencyType]*CircuitBreaker),
		}
		instance.initialize()
	})
	return instance
}

func (m *Manager) initialize() {
	m.breakers[DependencyRedis] = New(Config{
		Name:             string(DependencyRedis),
		FailureThreshold: constants.CircuitBreakerRedisFailureThreshold,
		SuccessThreshold: constants.CircuitBreakerDefaultSuccessThreshold,
		Timeout:          constants.CircuitBreakerRedisTimeout,
		OnStateChange:    m.createStateChangeHandler(DependencyRedis),
	})

	m.breakers[DependencyNATS] = New(Config{
		Name:             string(DependencyNATS),
		FailureThreshold: constants.CircuitBreakerNatsFailureThreshold,
		SuccessThreshold: constants.CircuitBreakerDefaultSuccessThreshold,
		Timeout:          constants.CircuitBreakerNatsTimeout,
		OnStateChange:    m.createStateChangeHandler(DependencyNATS),
	})

	for _, dep := range restDependencies {
		m.breakers[dep] = New(Config{
			Name:             string(dep),
			FailureThreshold: constants.CircuitBreakerRestFailureThreshold,
			SuccessThreshold: constants.CircuitBreakerRestSuccessThreshold,
			Timeout:          constants.CircuitBreakerRestTimeout,
			OnStateChange:    m.createStateChangeHandler(dep),
		})
	}
}

func (m *Manager) createStateChangeHandler(depType DependencyType) func(from, to State) {
	return func(from, to State) {
		logger := m.getLoggerForDependency(depType)

		switch to {
		case StateOpen:
			logger.Warn(fmt.Sprintf(
				string(constants.WarnCircuitBreakerOpen), string(depType),
			))
		case StateClosed:
			logger.Info(fmt.Sprintf(
				string(constants.InfoCircuitBreakerClosed), string(depType),
			))
		case StateHalfOpen:
			logger.Info(fmt.Sprintf(
				string(constants.InfoCircuitBreakerHalfOpen), string(depType),
			))
		default:
		}

		logger.Info(fmt.Sprintf(
			string(constants.InfoCircuitBreakerStateChange),
			string(depType), string(from), string(to),
		))
	}
}

func (*Manager) getLoggerForDependency(depType DependencyType) interface {
	Info(string)
	Warn(string)
	Error(string)
} {
	switch depType {
	case DependencyNATS:
		return constants.GetLogger(constants.LoggerPrefixEventPublisher)
	default:
		return constants.GetLogger(constants.LoggerCircuitBreaker)
	}
}

func (m *Manager) Execute(depType DependencyType, operation func() error) error {
	m.mu.RLock()
	breaker, exists := m.breakers[depType]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf(string(constants.ErrNoCircuitBreakerRegistered), string(depType))
	}

	return breaker.Execute(operation)
}

func (m *Manager) GetState(depType DependencyType) State {
	m.mu.RLock()
	breaker, exists := m.breakers[depType]
	m.mu.RUnlock()

	if !exists {
		return StateClosed
	}

	return breaker.GetState()
}

func (m *Manager) Reset(depType DependencyType) {
	m.mu.RLock()
	breaker, exists := m.breakers[depType]
	m.mu.RUnlock()

	if exists {
		breaker.Reset()
	}
}

func ExecuteRedis(operation func() error) error {
	return GetManager().Execute(DependencyRedis, operation)
}

func ExecuteNATS(operation func() error) error {
	return GetManager().Execute(DependencyNATS, operation)
}

func ExecuteExporter(operation func() error) error {
	return GetManager().Execute(DependencyExporter, operation)
}

func ExecuteEnrichment(operation func() error) error {
	return GetManager().Execute(DependencyEnrichment, operation)
}
