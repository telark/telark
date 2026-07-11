package metrics

import "time"

var Config = struct {
	// VelocityMinHistory is the minimum span between first and last changeLog detectedAt
	VelocityMinHistory time.Duration
}{
	VelocityMinHistory: time.Hour,
}
