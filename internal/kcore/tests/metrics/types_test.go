package metrics_test

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/types"
)

func TestContainerMetricsStruct(t *testing.T) {
	cm := types.ContainerMetrics{CPU: "100m", Memory: "128Mi", Pod: "pod1"}
	if cm.CPU != "100m" || cm.Memory != "128Mi" || cm.Pod != "pod1" {
		t.Error("ContainerMetrics struct fields not set correctly")
	}
}

func TestPodMetricsStruct(t *testing.T) {
	pm := types.PodMetrics{PodName: "pod1", Namespace: "ns", Containers: map[string]types.ContainerMetrics{}, TotalCPU: "100m", TotalMemory: "128Mi"}
	if pm.PodName != "pod1" || pm.Namespace != "ns" {
		t.Error("PodMetrics struct fields not set correctly")
	}
}

func TestMetricsClientStruct(t *testing.T) {
	mc := &types.MetricsClient{}
	if mc == nil {
		t.Error("MetricsClient struct not created")
	}
}

func TestMetricsAdapterStruct(t *testing.T) {
	ma := &types.MetricsAdapter{}
	if ma == nil {
		t.Error("MetricsAdapter struct not created")
	}
}
