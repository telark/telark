package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/data/logger"
	usage "github.com/plsyro/data/resources/application"
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
) *usage.Usage {
	ug := &usage.Usage{
		QoS:       qos,
		Timestamp: time.Now().Format(time.RFC3339),
		Available: false,
		Resources: usage.ResourceUsage{
			TotalCPU:         globalshared.DefaultCPU,
			TotalMemory:      globalshared.DefaultMemory,
			UsagePerInstance: []usage.UsagePerInstance{},
		},
	}

	metricsAdapter, err := client.NewMetricsAdapter()
	if err != nil {
		usageLogger.Error(fmt.Sprintf(string(constants.ErrFailedToInitializeMetricsClient), err))
		return ug
	}

	if !client.IsMetricsAvailable(metricsAdapter) {
		return ug
	}

	podMetricsList, err := client.GetAllPodMetrics(metricsAdapter, namespace, selectors)
	if err != nil {
		return ug
	}

	if len(podMetricsList) == constants.EmptySliceLength {
		return ug
	}

	ug.Available = true
	ug.Resources = buildResourceFromPodMetricsList(podMetricsList)

	return ug
}

func buildResourceFromPodMetricsList(
	podMetricsList []*metricstypes.PodMetrics,
) usage.ResourceUsage {
	instances := make(
		[]usage.UsagePerInstance,
		constants.EmptySliceLength,
		len(podMetricsList),
	)
	var totalCPU, totalMemory int64

	for _, podMetrics := range podMetricsList {
		instance := usage.UsagePerInstance{
			Name:       podMetrics.PodName,
			Containers: []usage.ContainerUsage{},
		}

		var instanceCPU, instanceMemory int64
		for containerName, containerMetrics := range podMetrics.Containers {
			containerUsage := usage.ContainerUsage{
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

	return usage.ResourceUsage{
		TotalCPU:         metricsutils.FormatCPU(totalCPU),
		TotalMemory:      metricsutils.FormatMemory(totalMemory),
		UsagePerInstance: instances,
	}
}
