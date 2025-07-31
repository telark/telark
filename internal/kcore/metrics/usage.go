package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/logging"
	workloadCommon "github.com/plsyro/data-pkg/resources/workload/common"
	"github.com/plsyro/kcore-pkg/constants"
	metricsClient "github.com/plsyro/kcore-pkg/metrics/client"
	types "github.com/plsyro/kcore-pkg/metrics/types"
	"github.com/plsyro/kcore-pkg/metrics/utils"
	subsAdapter "github.com/plsyro/kcore-pkg/resources/subs"
)

var usageLogger = logging.NewCustomLogger(constants.LOGGER_PREFIX_WORKLOAD_USAGE)

func GetWorkloadQoS(namespace string, selectors map[string]string) string {
	if namespace == "" || selectors == nil {
		return ""
	}

	qos, err := subsAdapter.GetQualityOfService(namespace, selectors)
	if err != nil {
		return ""
	}

	return qos
}

func BuildWorkloadUsage(namespace string, selectors map[string]string, qos string) *workloadCommon.Usage {
	usage := &workloadCommon.Usage{
		Qos:       qos,
		Timestamp: time.Now().Format(time.RFC3339),
		Available: false,
		Resources: workloadCommon.Resource{
			TotalCpu:         common.DEFAULT_CPU,
			TotalMemory:      common.DEFAULT_MEMORY,
			UsagePerInstance: []workloadCommon.UsagePerInstance{},
		},
	}

	metricsAdapter, err := metricsClient.NewMetricsAdapter()
	if err != nil {
		usageLogger.Warning(fmt.Sprintf(string(constants.ERROR_FAILED_TO_INITIALIZE_METRICS_CLIENT), err))
		return usage
	}

	if !metricsClient.IsMetricsAvailable(metricsAdapter) {
		usageLogger.Warning(string(constants.INFO_METRICS_API_UNAVAILABLE))
		return usage
	}

	podMetrics, err := metricsClient.GetFirstPodMetrics(metricsAdapter, namespace, selectors)
	if err != nil {
		usageLogger.Warning(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_POD_METRICS), err))
		return usage
	}

	if podMetrics == nil {
		usageLogger.Warning(string(constants.ERROR_FAILED_TO_GET_POD_METRICS))
		return usage
	}

	usage.Available = true
	usage.Resources = buildResourceFromPodMetrics(podMetrics)

	return usage
}

func buildResourceFromPodMetrics(podMetrics *types.PodMetrics) workloadCommon.Resource {
	var instances []workloadCommon.UsagePerInstance

	instance := workloadCommon.UsagePerInstance{
		Name:       podMetrics.PodName,
		Containers: []workloadCommon.ContainerUsage{},
	}

	var totalCpu, totalMemory int64
	for containerName, containerMetrics := range podMetrics.Containers {
		containerUsage := workloadCommon.ContainerUsage{
			Name:   containerName,
			Cpu:    containerMetrics.CPU,
			Memory: containerMetrics.Memory,
		}
		instance.Containers = append(instance.Containers, containerUsage)

		cpuValue := utils.ParseCPU(containerMetrics.CPU)
		memoryValue := utils.ParseMemory(containerMetrics.Memory)

		totalCpu += cpuValue
		totalMemory += memoryValue
	}

	instance.TotalCpu = utils.FormatCPU(totalCpu)
	instance.TotalMemory = utils.FormatMemory(totalMemory)

	instances = append(instances, instance)

	return workloadCommon.Resource{
		TotalCpu:         instance.TotalCpu,
		TotalMemory:      instance.TotalMemory,
		UsagePerInstance: instances,
	}
}
