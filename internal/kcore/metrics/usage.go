package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/logging"
	workloadCommon "github.com/plsyro/data-pkg/resources/workload/common"
	"github.com/plsyro/kcore-pkg/constants"
	metricsClient "github.com/plsyro/kcore-pkg/metrics/client"
	"github.com/plsyro/kcore-pkg/metrics/metricsutils"
	types "github.com/plsyro/kcore-pkg/metrics/types"
	"github.com/plsyro/kcore-pkg/resources/workload"
)

var usageLogger = logging.NewCustomLogger(constants.LoggerPrefixWorkloadUsage)

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
			TotalCPU:         common.DEFAULT_CPU,
			TotalMemory:      common.DEFAULT_MEMORY,
			UsagePerInstance: []workloadCommon.UsagePerInstance{},
		},
	}

	metricsAdapter, err := metricsClient.NewMetricsAdapter()
	if err != nil {
		usageLogger.Warning(fmt.Sprintf(string(constants.ErrFailedToInitializeMetricsClient), err))
		return usage
	}

	if !metricsClient.IsMetricsAvailable(metricsAdapter) {
		usageLogger.Warning(string(constants.InfoMetricsAPIUnavailable))
		return usage
	}

	podMetricsList, err := metricsClient.GetAllPodMetrics(metricsAdapter, namespace, selectors)
	if err != nil {
		usageLogger.Warning(fmt.Sprintf(string(constants.ErrFailedToGetPodMetrics), err))
		return usage
	}

	if len(podMetricsList) == 0 {
		usageLogger.Warning(string(constants.ErrFailedToGetPodMetrics))
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
