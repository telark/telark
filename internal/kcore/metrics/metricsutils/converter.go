package metricsutils

import (
	types "github.com/plsyro/kcore/metrics/types"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func ConvertToPodMetrics(pm *metricsv1beta1.PodMetrics) *types.PodMetrics {
	containers := make(map[string]types.ContainerMetrics, len(pm.Containers))
	var totalCPU, totalMemory int64

	for _, container := range pm.Containers {
		cpu := container.Usage.Cpu().MilliValue()
		memory := container.Usage.Memory().Value()

		containers[container.Name] = types.ContainerMetrics{
			CPU:    FormatCPU(cpu),
			Memory: FormatMemory(memory),
			Pod:    pm.Name,
		}

		totalCPU += cpu
		totalMemory += memory
	}

	return &types.PodMetrics{
		PodName:     pm.Name,
		Namespace:   pm.Namespace,
		Containers:  containers,
		TotalCPU:    FormatCPU(totalCPU),
		TotalMemory: FormatMemory(totalMemory),
	}
}
