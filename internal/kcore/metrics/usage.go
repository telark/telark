package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/data/logger"
	workloadCommon "github.com/plsyro/data/resources/workloads/shared"
	globalshared "github.com/plsyro/data/shared"
	"github.com/plsyro/kcore/constants"
	metricsClient "github.com/plsyro/kcore/metrics/client"
	"github.com/plsyro/kcore/metrics/metricsutils"
	types "github.com/plsyro/kcore/metrics/types"
	"github.com/plsyro/kcore/resources/workload"
)

var usageLogger = logger.NewCustomLogger(constants.LoggerPrefixWorkloadUsage)

func GetWorkloadQualityOfService(namespace string, selectors map[string]string) string {
	if namespace == "" || selectors == nil {
		return ""
	}

	qos, err := workload.GetQualityOfService(namespace, selectors)
	if err != nil {
		return ""
	}

	return qos
}

func BuildWorkloadUsage(namespace string, selectors map[string]string, qos string) *workloadCommon.Usage {
	usage := &workloadCommon.Usage{
		QoS:       qos,
		Timestamp: time.Now().Format(time.RFC3339),
		Available: false,
		Resources: workloadCommon.Resource{
			TotalCPU:         globalshared.DefaultCPU,
			TotalMemory:      globalshared.DefaultMemory,
			UsagePerInstance: []workloadCommon.UsagePerInstance{},
		},
	}

	metricsAdapter, err := metricsClient.NewMetricsAdapter()
	if err != nil {
		usageLogger.Error(fmt.Sprintf(string(constants.ErrFailedToInitializeMetricsClient), err))
		return usage
	}

	if !metricsClient.IsMetricsAvailable(metricsAdapter) {
		usageLogger.Warn(string(constants.InfoMetricsAPIUnavailable))
		return usage
	}

	podMetricsList, err := metricsClient.GetAllPodMetrics(metricsAdapter, namespace, selectors)
	if err != nil {
		usageLogger.Error(fmt.Sprintf(string(constants.ErrFailedToGetPodMetrics), err))
		return usage
	}

	if len(podMetricsList) == 0 {
		usageLogger.Warn(string(constants.ErrFailedToGetPodMetrics))
		return usage
	}

	usage.Available = true
	usage.Resources = buildResourceFromPodMetricsList(podMetricsList)

	return usage
}

func buildResourceFromPodMetricsList(podMetricsList []*types.PodMetrics) workloadCommon.Resource {
	instances := make([]workloadCommon.UsagePerInstance, 0, len(podMetricsList))
	var totalCPU, totalMemory int64

	for _, podMetrics := range podMetricsList {
		instance := workloadCommon.UsagePerInstance{
			Name:       podMetrics.PodName,
			Containers: []workloadCommon.ContainerUsage{},
		}

		var instanceCPU, instanceMemory int64
		for containerName, containerMetrics := range podMetrics.Containers {
			containerUsage := workloadCommon.ContainerUsage{
				Name:   containerName,
				CPU:    containerMetrics.CPU,
				Memory: containerMetrics.Memory,
			}
			instance.Containers = append(instance.Containers, containerUsage)

			cpuValue := metricsutils.ParseCPU(containerMetrics.CPU)
			memoryValue := metricsutils.ParseMemory(containerMetrics.Memory)

			instanceCPU += cpuValue
			instanceMemory += memoryValue
		}

		instance.TotalCPU = metricsutils.FormatCPU(instanceCPU)
		instance.TotalMemory = metricsutils.FormatMemory(instanceMemory)

		instances = append(instances, instance)

		// Add to total resources
		totalCPU += instanceCPU
		totalMemory += instanceMemory
	}

	return workloadCommon.Resource{
		TotalCPU:         metricsutils.FormatCPU(totalCPU),
		TotalMemory:      metricsutils.FormatMemory(totalMemory),
		UsagePerInstance: instances,
	}
}
