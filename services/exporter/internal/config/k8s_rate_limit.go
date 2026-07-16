package config

import (
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/k8sclient"
)

func ApplyKubernetesRESTRateLimit() {
	k8sclient.ApplyRESTClientRateLimitFromEnv(k8sclient.RateLimitEnv{
		QPSVar:       constants.EnvExporterK8sClientQPS,
		BurstVar:     constants.EnvExporterK8sClientBurst,
		DefaultQPS:   float64(constants.DefaultExporterK8sClientQPS),
		DefaultBurst: constants.DefaultExporterK8sClientBurst,
	})
}
