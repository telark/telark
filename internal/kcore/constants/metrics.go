package constants

import "time"

const (
	METRICS_CIRCUIT_BREAKER_MAX_FAILURES  = 5
	METRICS_CIRCUIT_BREAKER_TIMEOUT       = 30 * time.Second
	METRICS_CIRCUIT_BREAKER_RESET_TIMEOUT = 60 * time.Second
	CIRCUIT_BREAKER_IS_OPEN               = "circuit breaker is open"
	METRICS_API_RATE_LIMIT                = 1 * time.Second
	K8S_API_RATE_LIMIT                    = 500 * time.Millisecond
	DEFAULT_WORKER_POOL_SIZE              = 10
	KB                                    = 1024
	MB                                    = KB * 1024
	GB                                    = MB * 1024
	CPU_MILLICORE_THRESHOLD               = 1000
	CPU_CORE_DIVISOR                      = 1000.0
	DEPLOYMENT_READY_REPLICAS             = 1
)
