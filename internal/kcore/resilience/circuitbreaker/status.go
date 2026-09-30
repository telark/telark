package circuitbreaker

const (
	Closed CircuitBreakerState = iota
	Open
	HalfOpen
)
