package metrics

import (
	"fmt"
	"time"

	"github.com/plsyro/kcore-pkg/constants"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func (mc *MetricsClient) IsAvailable() bool {
	mc.mu.RLock()
	if time.Since(mc.lastCheck) <= mc.checkInterval {
		defer mc.mu.RUnlock()
		return mc.isAvailable
	}
	mc.mu.RUnlock()

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if time.Since(mc.lastCheck) > mc.checkInterval {
		mc.rateLimiter.Wait()
		mc.isAvailable = checkMetricsAPIAvailability(mc.client)
		mc.lastCheck = time.Now()

		if !mc.isAvailable {
			logger.Warning(string(constants.INFO_METRICS_API_UNAVAILABLE))
		}
	}

	return mc.isAvailable
}

func checkMetricsAPIAvailability(client *metricsclientset.Clientset) bool {
	if client == nil {
		return false
	}

	_, err := client.Discovery().ServerResourcesForGroupVersion(constants.METRICS_API_VERSION)
	if err != nil {
		logger.Error(fmt.Sprintf(string(constants.ERROR_METRICS_API_CHECK_FAILED), err))
		return false
	}

	return true
}
