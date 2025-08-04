package metrics

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/shared"
	metricstypes "github.com/plsyro/kcore-pkg/metrics/types"
	"github.com/plsyro/kcore-pkg/resilience/ratelimiting"
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
