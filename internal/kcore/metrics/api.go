package metrics

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/circuit_breaker"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func (mc *MetricsClient) GetPodMetrics(namespace, podName string) (*PodMetrics, error) {
	if !mc.IsAvailable() {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	mc.rateLimiter.Wait()

	var podMetrics *metricsv1beta1.PodMetrics
	err := mc.circuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeout(constants.METRICS_GET_TIMEOUT)
		defer cancel()

		var apiErr error
		podMetrics, apiErr = mc.client.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuit_breaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf(string(constants.INFO_METRICS_API_CIRCUIT_BREAKER_OPEN))
		}
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_TO_GET_POD_METRICS), err)
	}

	return convertToPodMetrics(podMetrics), nil
}

func (mc *MetricsClient) GetContainerMetrics(namespace, podName, containerName string) (*ContainerMetrics, error) {
	if !mc.IsAvailable() {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	podMetrics, err := mc.GetPodMetrics(namespace, podName)
	if err != nil {
		return nil, err
	}

	containerMetrics, exists := podMetrics.Containers[containerName]
	if !exists {
		return nil, fmt.Errorf(string(constants.ERROR_CONTAINER_NOT_FOUND_IN_POD), containerName, podName)
	}

	return &containerMetrics, nil
}

func (mc *MetricsClient) ListPodMetrics(namespace string) ([]*PodMetrics, error) {
	if !mc.IsAvailable() {
		return nil, fmt.Errorf(string(constants.INFO_METRICS_API_UNAVAILABLE))
	}

	mc.rateLimiter.Wait()

	var podMetricsList *metricsv1beta1.PodMetricsList
	err := mc.circuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeout(constants.METRICS_LIST_TIMEOUT)
		defer cancel()

		var apiErr error
		podMetricsList, apiErr = mc.client.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuit_breaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf(string(constants.INFO_METRICS_API_CIRCUIT_BREAKER_OPEN))
		}
		return nil, fmt.Errorf(string(constants.INFO_FAILED_TO_LIST_POD_METRICS), err)
	}

	var metrics []*PodMetrics
	for _, pm := range podMetricsList.Items {
		metrics = append(metrics, convertToPodMetrics(&pm))
	}

	return metrics, nil
}
