package config

import (
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/kcore/k8sclient"
)

func ApplyKubernetesRESTRateLimit() {
	k8sclient.ApplyRESTClientRateLimitFromEnv(k8sclient.RateLimitEnv{
		QPSVar:       constants.EnvDiscoveryK8sClientQPS,
		BurstVar:     constants.EnvDiscoveryK8sClientBurst,
		DefaultQPS:   float64(constants.DefaultDiscoveryK8sClientQPS),
		DefaultBurst: constants.DefaultDiscoveryK8sClientBurst,
	})
}

func RollbackK8sClientRateLimit() (float32, int) {
	return k8sclient.RateLimitFromEnv(k8sclient.RateLimitEnv{
		QPSVar:       constants.EnvDiscoveryRollbackK8sClientQPS,
		BurstVar:     constants.EnvDiscoveryRollbackK8sClientBurst,
		DefaultQPS:   float64(constants.DefaultRollbackK8sClientQPS),
		DefaultBurst: constants.DefaultRollbackK8sClientBurst,
	})
}
