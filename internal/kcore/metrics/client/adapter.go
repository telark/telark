package client

import (
	"fmt"
	"strings"

	"github.com/plsyro/kcore/constants"
	shared "github.com/plsyro/kcore/metrics/shared"
	types "github.com/plsyro/kcore/metrics/types"
)

func NewMetricsAdapter() (*types.MetricsAdapter, error) {
	client, err := InitMetricsClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToInitializeMetricsClient), err)
	}

	return &types.MetricsAdapter{
		Client: client,
	}, nil
}

func GetWorkloadMetrics(ma *types.MetricsAdapter, namespace string, selectors map[string]string) (map[string]*types.ContainerMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf("%s", constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
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

func GetAllPodMetrics(ma *types.MetricsAdapter, namespace string, selectors map[string]string) ([]*types.PodMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf("%s", constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
	}

	var matchingPods []*types.PodMetrics
	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			matchingPods = append(matchingPods, pm)
		}
	}

	if len(matchingPods) == 0 {
		return nil, fmt.Errorf("%s", constants.ErrNoPodsFoundMatchingSelectors)
	}

	return matchingPods, nil
}

func AdapterGetContainerMetrics(ma *types.MetricsAdapter, namespace, podName, containerName string) (*types.ContainerMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf("%s", constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
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
