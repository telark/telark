package metrics

import (
	"testing"

	"github.com/plsyro/kcore-pkg/metrics/types"
)

func TestContainerMetricsStruct(t *testing.T) {
	cm := types.ContainerMetrics{CPU: TestCPUString, Memory: TestMemoryStringMi, Pod: TestPodName}
	if cm.CPU != TestCPUString || cm.Memory != TestMemoryStringMi || cm.Pod != TestPodName {
		t.Error(ExpectedContainerMetricsStructFieldsNotSet)
	}
}

func TestPodMetricsStruct(t *testing.T) {
	pm := types.PodMetrics{
		PodName:     TestPodName,
		Namespace:   TestNamespace,
		Containers:  map[string]types.ContainerMetrics{},
		TotalCPU:    TestCPUString,
		TotalMemory: TestMemoryStringMi,
	}
	if pm.PodName != TestPodName || pm.Namespace != TestNamespace ||
		len(pm.Containers) != 0 || pm.TotalCPU != TestCPUString || pm.TotalMemory != TestMemoryStringMi {
		t.Error("PodMetrics struct fields not set correctly")
	}
}

func TestMetricsClientStruct(t *testing.T) {
	mc := &types.MetricsClient{}
	// Test that the struct can be created
	_ = mc
}

func TestMetricsAdapterStruct(t *testing.T) {
	ma := &types.MetricsAdapter{}
	// Test that the struct can be created
	_ = ma
}
