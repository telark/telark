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
