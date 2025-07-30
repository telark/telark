package metrics_test

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/client"
)

func TestInitMetricsClient(t *testing.T) {
	client.ResetClient()
	_, err := client.InitMetricsClient()
	if err != nil {
		t.Logf("InitMetricsClient returned error: %v", err)
	}
}

func TestResetClient(t *testing.T) {
	client.ResetClient()
}

func TestNewMetricsAdapter(t *testing.T) {
	adapter, err := client.NewMetricsAdapter()
	if err != nil && adapter != nil {
		t.Errorf("Expected nil adapter on error, got: %v", adapter)
	}
}

func TestGetWorkloadMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetWorkloadMetrics(nil, "ns", map[string]string{"app": "test"})
	if err == nil {
		t.Error("Expected error for nil adapter")
	}
}

func TestGetFirstPodMetrics_NilAdapter(t *testing.T) {
	_, err := client.GetFirstPodMetrics(nil, "ns", map[string]string{"app": "test"})
	if err == nil {
		t.Error("Expected error for nil adapter")
	}
}

func TestAdapterGetContainerMetrics_NilAdapter(t *testing.T) {
	_, err := client.AdapterGetContainerMetrics(nil, "ns", "pod", "container")
	if err == nil {
		t.Error("Expected error for nil adapter")
	}
}
