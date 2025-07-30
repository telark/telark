package metrics

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/shared"
	"github.com/plsyro/kcore-pkg/metrics/types"
	"github.com/plsyro/kcore-pkg/resilience/rate_limiting"
)

func TestIsClientAvailable_Nil(t *testing.T) {
	var mc *types.MetricsClient
	if shared.IsClientAvailable(mc) {
		t.Error(ExpectedFalseForNilMetricsClient)
	}
}

func TestIsClientAvailable_Default(t *testing.T) {
	mc := &types.MetricsClient{
		RateLimiter: rate_limiting.NewRateLimiter(1),
	}
	_ = shared.IsClientAvailable(mc) // Should not panic
}
