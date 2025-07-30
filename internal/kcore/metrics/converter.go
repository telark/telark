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

// FormatCPU formats CPU millicores to string representation
func FormatCPU(milliValue int64) string {
	if milliValue < constants.CPU_MILLICORE_THRESHOLD {
		return fmt.Sprintf("%dm", milliValue)
	}
	return fmt.Sprintf("%.2f", float64(milliValue)/constants.CPU_CORE_DIVISOR)
}

// FormatMemory formats memory bytes to string representation
func FormatMemory(bytes int64) string {
	switch {
	case bytes >= constants.GB:
		return formatMemoryUnit(bytes, constants.GB, "Gi")
	case bytes >= constants.MB:
		return formatMemoryUnit(bytes, constants.MB, "Mi")
	case bytes >= constants.KB:
		return formatMemoryUnit(bytes, constants.KB, "Ki")
	default:
		return formatMemoryUnit(bytes, 1, "B")
	}
}

func formatMemoryUnit(bytes, unit int64, suffix string) string {
	if unit == 1 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.2f%s", float64(bytes)/float64(unit), suffix)
}
