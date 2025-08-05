package metrics

import (
	"testing"

	metricstypes "github.com/plsyro/kcore/metrics/types"
)

func TestContainerMetricsStruct(t *testing.T) {
	cm := metricstypes.ContainerMetrics{CPU: TestCPUString, Memory: TestMemoryStringMi, Pod: TestPodName}
	if cm.CPU != TestCPUString || cm.Memory != TestMemoryStringMi || cm.Pod != TestPodName {
		t.Error(ExpectedContainerMetricsStructFieldsNotSet)
	}
}

func TestPodMetricsStruct(t *testing.T) {
	pm := metricstypes.PodMetrics{
		PodName:     TestPodName,
		Namespace:   TestNamespace,
		Containers:  map[string]metricstypes.ContainerMetrics{},
		TotalCPU:    TestCPUString,
		TotalMemory: TestMemoryStringMi,
	}
	if pm.PodName != TestPodName || pm.Namespace != TestNamespace ||
		len(pm.Containers) != 0 || pm.TotalCPU != TestCPUString || pm.TotalMemory != TestMemoryStringMi {
		t.Error("PodMetrics struct fields not set correctly")
	}
}

func TestMetricsClientStruct(_ *testing.T) {
	mc := &metricstypes.MetricsClient{}
	// Test that the struct can be created
	_ = mc
}

func TestMetricsAdapterStruct(_ *testing.T) {
	ma := &metricstypes.MetricsAdapter{}
	// Test that the struct can be created
	_ = ma
}
