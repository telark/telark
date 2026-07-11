package metrics

const (
	// VelocityRoundFactor rounds changeVelocityPerDay to 2 decimals: math.Round(v*factor)/factor.
	VelocityRoundFactor = 100.0
	// HoursPerDay converts a duration in hours to fractional days for velocity.
	HoursPerDay = 24.0
)
