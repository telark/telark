package constants

import "time"

const (
	MetricsCircuitBreakerMaxFailures  = 5
	MetricsCircuitBreakerTimeout      = 30 * time.Second
	MetricsCircuitBreakerResetTimeout = 60 * time.Second
	CircuitBreakerIsOpen              = "circuit breaker is open"
	MetricsAPIRateLimit               = 1 * time.Second
	K8sAPIRateLimit                   = 500 * time.Millisecond
	DefaultWorkerPoolSize             = 10
	KB                                = 1024
	MB                                = KB * 1024
	GB                                = MB * 1024
	CPUMillicoreThreshold             = 1000
	CPUCoreDivisor                    = 1000.0
	DeploymentReadyReplicas           = 1
)
