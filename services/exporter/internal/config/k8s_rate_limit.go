package config

import (
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func ApplyKubernetesRESTRateLimit() {
	k8sclient.ApplyRESTClientRateLimitFromEnv(k8sclient.RateLimitEnv{
		QPSVar:       constants.EnvExporterK8sClientQPS,
		BurstVar:     constants.EnvExporterK8sClientBurst,
		DefaultQPS:   float64(constants.DefaultExporterK8sClientQPS),
		DefaultBurst: constants.DefaultExporterK8sClientBurst,
	})
}
