package metrics

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func convertToPodMetrics(pm *metricsv1beta1.PodMetrics) *PodMetrics {
	containers := make(map[string]ContainerMetrics, len(pm.Containers))
	var totalCPU, totalMemory int64

	for _, container := range pm.Containers {
		cpu := container.Usage.Cpu().MilliValue()
		memory := container.Usage.Memory().Value()

		containers[container.Name] = ContainerMetrics{
			CPU:    FormatCPU(cpu),
			Memory: FormatMemory(memory),
			Pod:    pm.Name,
		}

		totalCPU += cpu
		totalMemory += memory
	}

	return &PodMetrics{
		PodName:     pm.Name,
		Namespace:   pm.Namespace,
		Containers:  containers,
		TotalCPU:    FormatCPU(totalCPU),
		TotalMemory: FormatMemory(totalMemory),
	}
}

func FormatCPU(milliValue int64) string {
	if milliValue < constants.CPU_MILLICORE_THRESHOLD {
		return fmt.Sprintf(constants.CPU_MILLICORE_FORMAT, milliValue)
	}
	return fmt.Sprintf(constants.CPU_CORE_FORMAT, float64(milliValue)/constants.CPU_CORE_DIVISOR)
}

func FormatMemory(bytes int64) string {
	switch {
	case bytes >= constants.GB:
		return formatMemoryUnit(bytes, constants.GB, constants.MEMORY_UNIT_GI)
	case bytes >= constants.MB:
		return formatMemoryUnit(bytes, constants.MB, constants.MEMORY_UNIT_MI)
	case bytes >= constants.KB:
		return formatMemoryUnit(bytes, constants.KB, constants.MEMORY_UNIT_KI)
	default:
		return formatMemoryUnit(bytes, 1, constants.MEMORY_UNIT_B)
	}
}

func formatMemoryUnit(bytes, unit int64, suffix string) string {
	if unit == 1 {
		return fmt.Sprintf(constants.MEMORY_BYTES_FORMAT, bytes)
	}
	return fmt.Sprintf(constants.MEMORY_UNIT_FORMAT, float64(bytes)/float64(unit), suffix)
}
