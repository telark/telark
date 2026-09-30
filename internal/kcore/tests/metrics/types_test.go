package metrics

import (
	"testing"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/metrics/metricstypes"
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
		len(pm.Containers) != constants.EmptySliceLength || pm.TotalCPU != TestCPUString ||
		pm.TotalMemory != TestMemoryStringMi {
		t.Error(ExpectedPodMetricsStructFieldsNotSet)
	}
}

func TestMetricsClientStruct(_ *testing.T) {
	_ = &metricstypes.MetricsClient{}
}

func TestMetricsAdapterStruct(_ *testing.T) {
	_ = &metricstypes.MetricsAdapter{}
}
