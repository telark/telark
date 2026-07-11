package metrics

import (
	"testing"

	"github.com/telark/kcore/metrics/metricstypes"
	"github.com/telark/kcore/metrics/shared"
	"github.com/telark/kcore/resilience/ratelimiting"
)

func TestIsClientAvailable_Nil(t *testing.T) {
	var mc *metricstypes.MetricsClient
	if shared.IsClientAvailable(mc) {
		t.Error(ExpectedFalseForNilMetricsClient)
	}
}

func TestIsClientAvailable_Default(t *testing.T) {
	mc := &metricstypes.MetricsClient{
		RateLimiter: ratelimiting.NewRateLimiter(1),
	}
	_ = shared.IsClientAvailable(mc) // Should not panic
}
