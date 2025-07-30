package metrics_test

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/shared"
	"github.com/plsyro/kcore-pkg/metrics/types"
)

func TestIsClientAvailable_Nil(t *testing.T) {
	var mc *types.MetricsClient
	if shared.IsClientAvailable(mc) {
		t.Error("Expected false for nil MetricsClient")
	}
}

func TestIsClientAvailable_Default(t *testing.T) {
	mc := &types.MetricsClient{}
	_ = shared.IsClientAvailable(mc) // Should not panic
}
