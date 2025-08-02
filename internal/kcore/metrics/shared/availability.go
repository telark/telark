package shared

import (
	"time"

	"github.com/plsyro/kcore-pkg/constants"
	types "github.com/plsyro/kcore-pkg/metrics/types"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

func IsClientAvailable(mc *types.MetricsClient) bool {
	if mc == nil {
		return false
	}
	mc.Mu.RLock()
	if time.Since(mc.LastCheck) <= mc.CheckInterval {
		defer mc.Mu.RUnlock()
		return mc.Available
	}
	mc.Mu.RUnlock()

	mc.Mu.Lock()
	defer mc.Mu.Unlock()

	if time.Since(mc.LastCheck) > mc.CheckInterval {
		mc.RateLimiter.Wait()
		mc.Available = checkMetricsAPIAvailability(mc.Client)
		mc.LastCheck = time.Now()
	}

	return mc.Available
}

func checkMetricsAPIAvailability(client *metricsclientset.Clientset) bool {
	if client == nil {
		return false
	}

	_, err := client.Discovery().ServerResourcesForGroupVersion(constants.METRICS_API_VERSION)
	return err == nil
}
