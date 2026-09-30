package metrics

import (
	"testing"

	"github.com/telark/telark/internal/kcore/metrics/metricstypes"
	"github.com/telark/telark/internal/kcore/metrics/shared"
	"github.com/telark/telark/internal/kcore/resilience/ratelimiting"
)

func TestIsClientAvailable_Nil(t *testing.T) {
	var mc *metricstypes.MetricsClient
	if shared.IsClientAvailable(mc) {
		t.Error(ExpectedFalseForNilMetricsClient)
	}
}

func TestIsClientAvailable_Default(t *testing.T) {
	mc := &metricstypes.MetricsClient{
		RateLimiter: ratelimiting.NewRateLimiter(TestRateLimitInterval),
	}
	if shared.IsClientAvailable(mc) {
		t.Error(ExpectedFalseForUnreachableMetricsAPI)
	}
}
