package client

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/metrics/metricstypes"
	"github.com/telark/kcore/metrics/metricsutils"
	"github.com/telark/kcore/metrics/shared"
	"github.com/telark/kcore/resilience/circuitbreaker"
	"github.com/telark/kcore/resilience/timeout"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func GetPodMetrics(
	mc *metricstypes.MetricsClient,
	namespace, podName string,
) (*metricstypes.PodMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPIUnavailable)
	}

	mc.RateLimiter.Wait()

	var podMetrics *metricsv1beta1.PodMetrics
	err := mc.CircuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeoutCause(constants.MetricsGetTimeout)
		defer cancel()

		var apiErr error
		podMetrics, apiErr = mc.Client.MetricsV1beta1().PodMetricses(namespace).Get(
			ctx, podName, metav1.GetOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPICircuitBreakerOpen)
		}
		return nil, fmt.Errorf(string(constants.ErrFailedToGetPodMetrics), err)
	}

	return metricsutils.ConvertToPodMetrics(podMetrics), nil
}

func GetContainerMetricsAPI(
	mc *metricstypes.MetricsClient,
	namespace, podName, containerName string,
) (*metricstypes.ContainerMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPIUnavailable)
	}

	podMetrics, err := GetPodMetrics(mc, namespace, podName)
	if err != nil {
		return nil, err
	}

	containerMetrics, exists := podMetrics.Containers[containerName]
	if !exists {
		return nil, fmt.Errorf(string(constants.ErrContainerNotFoundInPod), containerName, podName)
	}

	return &containerMetrics, nil
}

func ListPodMetrics(mc *metricstypes.MetricsClient, namespace string) ([]*metricstypes.PodMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPIUnavailable)
	}

	mc.RateLimiter.Wait()

	var podMetricsList *metricsv1beta1.PodMetricsList
	err := mc.CircuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeoutCause(constants.MetricsListTimeout)
		defer cancel()

		var apiErr error
		podMetricsList, apiErr = mc.Client.MetricsV1beta1().PodMetricses(namespace).List(
			ctx, metav1.ListOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuitbreaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf(constants.ErrorFormatString, constants.InfoMetricsAPICircuitBreakerOpen)
		}
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
	}

	metricsList := make([]*metricstypes.PodMetrics, constants.EmptySliceLength,
		len(podMetricsList.Items))
	for i := range podMetricsList.Items {
		metricsList = append(metricsList, metricsutils.ConvertToPodMetrics(&podMetricsList.Items[i]))
	}

	return metricsList, nil
}
