package circuit_breaker

const (
	CLOSED CircuitBreakerState = iota
	OPEN
	HALF_OPEN
)
