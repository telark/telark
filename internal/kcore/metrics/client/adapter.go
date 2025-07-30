package client

import (
	"fmt"
	"strings"

	"github.com/plsyro/kcore-pkg/constants"
	shared "github.com/plsyro/kcore-pkg/metrics/shared"
	types "github.com/plsyro/kcore-pkg/metrics/types"
)

func NewMetricsAdapter() (*types.MetricsAdapter, error) {
	client, err := InitMetricsClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_TO_INITIALIZE_METRICS_CLIENT), err)
	}

	return &types.MetricsAdapter{
		Client: client,
	}, nil
}

func GetWorkloadMetrics(ma *types.MetricsAdapter, namespace string, selectors map[string]string) (map[string]*types.ContainerMetrics, error) {
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.INFO_FAILED_TO_LIST_POD_METRICS), err)
	}

	workloadMetrics := make(map[string]*types.ContainerMetrics)
	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			for containerName, containerMetrics := range pm.Containers {
				workloadMetrics[containerName] = &containerMetrics
			}
		}
	}

	return workloadMetrics, nil
}

func GetFirstPodMetrics(ma *types.MetricsAdapter, namespace string, selectors map[string]string) (*types.PodMetrics, error) {
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
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

func AdapterGetContainerMetrics(ma *types.MetricsAdapter, namespace, podName, containerName string) (*types.ContainerMetrics, error) {
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	return GetContainerMetricsAPI(ma.Client, namespace, podName, containerName)
}

func IsMetricsAvailable(ma *types.MetricsAdapter) bool {
	return shared.IsClientAvailable(ma.Client)
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
