package client

import (
	"fmt"
	"strings"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/metrics/metricstypes"
	"github.com/plsyro/kcore/metrics/shared"
)

func NewMetricsAdapter() (*metricstypes.MetricsAdapter, error) {
	client, err := InitMetricsClient()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToInitializeMetricsClient), err)
	}

	return &metricstypes.MetricsAdapter{
		Client: client,
	}, nil
}

func GetWorkloadMetrics(
	ma *metricstypes.MetricsAdapter,
	namespace string,
	selectors map[string]string,
) (map[string]*metricstypes.ContainerMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
	}

	workloadMetrics := make(map[string]*metricstypes.ContainerMetrics)
	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			for containerName, containerMetrics := range pm.Containers {
				workloadMetrics[containerName] = &containerMetrics
			}
		}
	}

	return workloadMetrics, nil
}

func GetAllPodMetrics(
	ma *metricstypes.MetricsAdapter,
	namespace string,
	selectors map[string]string,
) ([]*metricstypes.PodMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	podMetrics, err := ListPodMetrics(ma.Client, namespace)
	if err != nil {
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
	}

	var matchingPods []*metricstypes.PodMetrics
	for _, pm := range podMetrics {
		if matchesSelectors(pm.PodName, selectors) {
			matchingPods = append(matchingPods, pm)
		}
	}

	if len(matchingPods) == constants.EmptySliceLength {
		return nil, fmt.Errorf(constants.ErrorFormatString,
			constants.ErrNoPodsFoundMatchingSelectors)
	}

	return matchingPods, nil
}

func AdapterGetContainerMetrics(
	ma *metricstypes.MetricsAdapter,
	namespace, podName, containerName string,
) (*metricstypes.ContainerMetrics, error) {
	if ma == nil || ma.Client == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrMetricsAdapterOrClientNil)
	}
	if !shared.IsClientAvailable(ma.Client) {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPIUnavailable)
	}

	return GetContainerMetricsAPI(ma.Client, namespace, podName, containerName)
}

func IsMetricsAvailable(ma *metricstypes.MetricsAdapter) bool {
	return shared.IsClientAvailable(ma.Client)
}

func matchesSelectors(podName string, selectors map[string]string) bool {
	if len(selectors) == constants.EmptySliceLength {
		return false
	}

	for _, value := range selectors {
		if !strings.Contains(podName, value) {
			return false
		}
	}
	return true
}
