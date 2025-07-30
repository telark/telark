package circuit_breaker

import "time"

const (
	METRICS_CIRCUIT_BREAKER_MAX_FAILURES                      = 5
	METRICS_CIRCUIT_BREAKER_TIMEOUT                           = 30 * time.Second
	METRICS_CIRCUIT_BREAKER_RESET_TIMEOUT                     = 60 * time.Second
	CIRCUIT_BREAKER_IS_OPEN                                   = "circuit breaker is open"
	CLOSED                                CircuitBreakerState = iota
	OPEN
	HALF_OPEN
)
