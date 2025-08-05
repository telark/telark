package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/data/logger"
	workloadshared "github.com/plsyro/data/resources/workloads/shared"
	globalshared "github.com/plsyro/data/shared"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/metrics/client"
	"github.com/plsyro/kcore/metrics/metricstypes"
	"github.com/plsyro/kcore/metrics/metricsutils"
	"github.com/plsyro/kcore/resources/workload"
)

var usageLogger = logger.NewCustomLogger(constants.LoggerPrefixWorkloadUsage)

func GetWorkloadQualityOfService(namespace string, selectors map[string]string) string {
	if namespace == constants.EmptyString || selectors == nil {
		return constants.EmptyString
	}

	qos, err := workload.GetQualityOfService(namespace, selectors)
	if err != nil {
		return constants.EmptyString
	}

	return qos
}

func BuildWorkloadUsage(
	namespace, qos string,
	selectors map[string]string,
) *workloadshared.Usage {
	usage := &workloadshared.Usage{
		QoS:       qos,
		Timestamp: time.Now().Format(time.RFC3339),
		Available: false,
		Resources: workloadshared.Resource{
			TotalCPU:         globalshared.DefaultCPU,
			TotalMemory:      globalshared.DefaultMemory,
			UsagePerInstance: []workloadshared.UsagePerInstance{},
		},
	}

	metricsAdapter, err := client.NewMetricsAdapter()
	if err != nil {
		usageLogger.Error(fmt.Sprintf(string(constants.ErrFailedToInitializeMetricsClient), err))
		return usage
	}

	if !client.IsMetricsAvailable(metricsAdapter) {
		usageLogger.Warn(string(constants.InfoMetricsAPIUnavailable))
		return usage
	}

	podMetricsList, err := client.GetAllPodMetrics(metricsAdapter, namespace, selectors)
	if err != nil {
		usageLogger.Error(fmt.Sprintf(string(constants.ErrFailedToGetPodMetrics), err))
		return usage
	}

	if len(podMetricsList) == constants.EmptySliceLength {
		usageLogger.Warn(string(constants.ErrFailedToGetPodMetrics))
		return usage
	}

	usage.Available = true
	usage.Resources = buildResourceFromPodMetricsList(podMetricsList)

	return usage
}

func buildResourceFromPodMetricsList(podMetricsList []*metricstypes.PodMetrics) workloadshared.Resource {
	instances := make(
		[]workloadshared.UsagePerInstance,
		constants.EmptySliceLength,
		len(podMetricsList),
	)
	var totalCPU, totalMemory int64

	for _, podMetrics := range podMetricsList {
		instance := workloadshared.UsagePerInstance{
			Name:       podMetrics.PodName,
			Containers: []workloadshared.ContainerUsage{},
		}

		var instanceCPU, instanceMemory int64
		for containerName, containerMetrics := range podMetrics.Containers {
			containerUsage := workloadshared.ContainerUsage{
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

	return workloadshared.Resource{
		TotalCPU:         metricsutils.FormatCPU(totalCPU),
		TotalMemory:      metricsutils.FormatMemory(totalMemory),
		UsagePerInstance: instances,
	}
}
