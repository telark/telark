package constants

import "time"

const (
	// Circuit Breaker constants
	METRICS_CIRCUIT_BREAKER_MAX_FAILURES  = 5
	METRICS_CIRCUIT_BREAKER_TIMEOUT       = 30 * time.Second
	METRICS_CIRCUIT_BREAKER_RESET_TIMEOUT = 60 * time.Second
	CIRCUIT_BREAKER_IS_OPEN               = "circuit breaker is open"

	// Rate Limiting constants
	METRICS_API_RATE_LIMIT = 1 * time.Second
	K8S_API_RATE_LIMIT     = 500 * time.Millisecond

	// Worker Pool constants
	DEFAULT_WORKER_POOL_SIZE = 10
)
