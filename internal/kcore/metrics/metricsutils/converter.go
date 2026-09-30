package metricsutils

import (
	"github.com/telark/kcore/metrics/metricstypes"
	k8smetrics "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func ConvertToPodMetrics(pm *k8smetrics.PodMetrics) *metricstypes.PodMetrics {
	containers := make(map[string]metricstypes.ContainerMetrics, len(pm.Containers))
	var totalCPU, totalMemory int64

	for _, container := range pm.Containers {
		cpu := container.Usage.Cpu().MilliValue()
		memory := container.Usage.Memory().Value()

		containers[container.Name] = metricstypes.ContainerMetrics{
			CPU:    FormatCPU(cpu),
			Memory: FormatMemory(memory),
			Pod:    pm.Name,
		}

		totalCPU += cpu
		totalMemory += memory
	}

	return &metricstypes.PodMetrics{
		PodName:     pm.Name,
		Namespace:   pm.Namespace,
		Containers:  containers,
		TotalCPU:    FormatCPU(totalCPU),
		TotalMemory: FormatMemory(totalMemory),
	}
}
