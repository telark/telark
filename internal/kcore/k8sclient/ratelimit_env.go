package k8sclient

import (
	"os"
	"strconv"
	"strings"

	"github.com/telark/kcore/constants"
)

const minRateLimitValue = 0

// RateLimitEnv names the variables a service reads its client rate limit from,
// and the values to use when they are unset. Each service tunes its own limit,
// so the names differ while the reading does not.
type RateLimitEnv struct {
	QPSVar       string
	BurstVar     string
	DefaultQPS   float64
	DefaultBurst int
}

// ApplyRESTClientRateLimitFromEnv sets the shared client's rate limit from the
// environment, keeping the default whenever a variable is unset or unusable.
func ApplyRESTClientRateLimitFromEnv(env RateLimitEnv) {
	SetRESTClientRateLimit(float32(qpsFromEnv(env)), burstFromEnv(env))
}

// RateLimitFromEnv reads a limit the same way, for a client that keeps its own
// budget instead of configuring the shared one.
func RateLimitFromEnv(env RateLimitEnv) (float32, int) {
	return float32(qpsFromEnv(env)), burstFromEnv(env)
}

func qpsFromEnv(env RateLimitEnv) float64 {
	raw := strings.TrimSpace(os.Getenv(env.QPSVar))
	if raw == constants.EmptyString {
		return env.DefaultQPS
	}

	value, err := strconv.ParseFloat(raw, constants.Base64)
	if err != nil || value <= minRateLimitValue {
		return env.DefaultQPS
	}

	return value
}

func burstFromEnv(env RateLimitEnv) int {
	raw := strings.TrimSpace(os.Getenv(env.BurstVar))
	if raw == constants.EmptyString {
		return env.DefaultBurst
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= minRateLimitValue {
		return env.DefaultBurst
	}

	return value
}
