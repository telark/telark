package metrics

import (
	"fmt"
	"strings"

	"github.com/plsyro/kcore-pkg/constants"
)

func NewMetricsAdapter() (*MetricsAdapter, error) {
	client, err := InitMetricsClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_TO_INITIALIZE_METRICS_CLIENT), err)
	}

	return &MetricsAdapter{
		client: client,
	}, nil
}

func (ma *MetricsAdapter) GetWorkloadMetrics(namespace string, selectors map[string]string) (map[string]*ContainerMetrics, error) {
	if !ma.client.IsAvailable() {
		logger.Warning(string(constants.INFO_METRICS_API_UNAVAILABLE))
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	podMetrics, err := ma.client.ListPodMetrics(namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.INFO_FAILED_TO_LIST_POD_METRICS), err)
	}

	workloadMetrics := make(map[string]*ContainerMetrics)
	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			for containerName, containerMetrics := range pm.Containers {
				workloadMetrics[containerName] = &containerMetrics
			}
		}
	}

	return workloadMetrics, nil
}

func (ma *MetricsAdapter) GetFirstPodMetrics(namespace string, selectors map[string]string) (*PodMetrics, error) {
	if !ma.client.IsAvailable() {
		logger.Warning(string(constants.INFO_METRICS_API_UNAVAILABLE))
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	podMetrics, err := ma.client.ListPodMetrics(namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.INFO_FAILED_TO_LIST_POD_METRICS), err)
	}

	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			return pm, nil
		}
	}

	return nil, fmt.Errorf(string(constants.ERROR_NO_PODS_FOUND_MATCHING_SELECTORS))
}

func (ma *MetricsAdapter) GetContainerMetrics(namespace, podName, containerName string) (*ContainerMetrics, error) {
	if !ma.client.IsAvailable() {
		logger.Warning(string(constants.INFO_METRICS_API_UNAVAILABLE))
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	return ma.client.GetContainerMetrics(namespace, podName, containerName)
}

func (ma *MetricsAdapter) IsMetricsAvailable() bool {
	return ma.client.IsAvailable()
}

func matchesSelectors(podName string, selectors map[string]string) bool {
	if len(selectors) == 0 {
		return false
	}

	for _, value := range selectors {
		if !strings.Contains(podName, value) {
			return false
		}
	}
	return true
}
