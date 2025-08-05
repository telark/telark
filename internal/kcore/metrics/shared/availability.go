package shared

import (
	"time"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/metrics/metricstypes"
	k8smetricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

func IsClientAvailable(mc *metricstypes.MetricsClient) bool {
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

func checkMetricsAPIAvailability(client *k8smetricsclient.Clientset) bool {
	if client == nil {
		return false
	}

	_, err := client.Discovery().ServerResourcesForGroupVersion(constants.MetricsAPIVersion)
	return err == nil
}
