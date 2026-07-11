package state

import "time"

const (
	HealthCheckPort          = "8080"
	HealthCheckHost          = "localhost"
	MaxRestartAttempts       = 10
	HealthCheckTimeout       = 5 * time.Second
	RestartDelay             = 5 * time.Second
	ShutdownTimeout          = 10 * time.Second
	HealthCheckInterval = 2 * time.Minute
)
