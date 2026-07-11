package circuitbreaker

import (
	"sync"
	"time"
)

type (
	State          string
	DependencyType string
	Manager        struct {
		breakers map[DependencyType]*CircuitBreaker
		mu       sync.RWMutex
	}
)

const (
	StateClosed     State          = "Closed"
	StateOpen       State          = "Open"
	StateHalfOpen   State          = "HalfOpen"
	DependencyRedis DependencyType = "Redis"
	DependencyNATS  DependencyType = "NATS"
)

type CircuitBreaker struct {
	name             string
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	failureCount     int
	successCount     int
	state            State
	lastFailureTime  time.Time
	lastStateChange  time.Time
	mutex            sync.RWMutex
	onStateChange    func(from, to State)
}

type Config struct {
	Name             string
	FailureThreshold int
	SuccessThreshold int
	Timeout          time.Duration
	OnStateChange    func(from, to State)
}

type Stats struct {
	State           State
	FailureCount    int
	SuccessCount    int
	LastStateChange time.Time
}
