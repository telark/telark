package client

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	shared "github.com/plsyro/kcore-pkg/metrics/shared"
	types "github.com/plsyro/kcore-pkg/metrics/types"
	utils "github.com/plsyro/kcore-pkg/metrics/utils"
	"github.com/plsyro/kcore-pkg/resilience/circuit_breaker"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func GetPodMetrics(mc *types.MetricsClient, namespace, podName string) (*types.PodMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	mc.RateLimiter.Wait()

	var podMetrics *metricsv1beta1.PodMetrics
	err := mc.CircuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeout(constants.MetricsGetTimeout)
		defer cancel()

		var apiErr error
		podMetrics, apiErr = mc.Client.MetricsV1beta1().PodMetricses(namespace).Get(ctx, podName, metav1.GetOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuit_breaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf("%s", constants.InfoMetricsAPICircuitBreakerOpen)
		}
		return nil, fmt.Errorf(string(constants.ErrFailedToGetPodMetrics), err)
	}

	return utils.ConvertToPodMetrics(podMetrics), nil
}

func GetContainerMetricsAPI(mc *types.MetricsClient, namespace, podName, containerName string) (*types.ContainerMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
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

func ListPodMetrics(mc *types.MetricsClient, namespace string) ([]*types.PodMetrics, error) {
	if !shared.IsClientAvailable(mc) {
		return nil, fmt.Errorf("%s", constants.InfoMetricsAPIUnavailable)
	}

	mc.RateLimiter.Wait()

	var podMetricsList *metricsv1beta1.PodMetricsList
	err := mc.CircuitBreaker.Call(func() error {
		ctx, cancel := timeout.ContextWithTimeout(constants.MetricsListTimeout)
		defer cancel()

		var apiErr error
		podMetricsList, apiErr = mc.Client.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
		return apiErr
	})
	if err != nil {
		if err == circuit_breaker.ErrCircuitBreakerOpen {
			return nil, fmt.Errorf("%s", constants.InfoMetricsAPICircuitBreakerOpen)
		}
		return nil, fmt.Errorf(string(constants.InfoFailedToListPodMetrics), err)
	}

	var metricsList []*types.PodMetrics
	for _, pm := range podMetricsList.Items {
		metricsList = append(metricsList, utils.ConvertToPodMetrics(&pm))
	}

	return metricsList, nil
}
