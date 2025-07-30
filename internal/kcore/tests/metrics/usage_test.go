package metrics_test

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics"
)

func TestGetWorkloadQoS_EmptyNamespace(t *testing.T) {
	qos := metrics.GetWorkloadQoS("", nil)
	if qos != "" {
		t.Errorf("Expected empty string for empty namespace, got %q", qos)
	}
}

func TestBuildWorkloadUsage_EmptyNamespace(t *testing.T) {
	usage := metrics.BuildWorkloadUsage("", nil, "")
	if usage != nil && usage.Available {
		t.Error("Expected usage to be unavailable for empty namespace")
	}
}
