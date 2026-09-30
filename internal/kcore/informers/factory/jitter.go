package factory

import (
	"math/rand/v2"
	"time"

	"github.com/telark/kcore/constants"
)

// JitteredResync returns base perturbed by ±(base*fraction) so multiple replicas
// do not stampede LIST calls in the same window. fraction is clamped to
// [0, 0.5]. A non-positive base or zero fraction returns base unchanged.
func JitteredResync(base time.Duration, fraction float64) time.Duration {
	if base <= constants.ZeroValue {
		return base
	}
	if fraction <= constants.ZeroJitterFraction {
		return base
	}
	if fraction > maxJitterFraction {
		fraction = maxJitterFraction
	}
	spread := float64(base) * fraction
	// Scheduling jitter, not cryptographic; math/rand/v2 is correct here.
	//nolint:gosec // G404: non-security use.
	signedUnit := rand.Float64()*constants.JitterRangeMultiplier - constants.JitterCenterOffset
	return base + time.Duration(signedUnit*spread)
}

const maxJitterFraction = 0.5
