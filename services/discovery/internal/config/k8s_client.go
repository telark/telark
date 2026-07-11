package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/kcore/k8sclient"
)

func ApplyKubernetesRESTRateLimit() {
	qps := float64(constants.DefaultDiscoveryK8sClientQPS)
	if raw := strings.TrimSpace(os.Getenv(constants.EnvDiscoveryK8sClientQPS)); raw != constants.EmptyString {
		if v, err := strconv.ParseFloat(raw, constants.Float64ParseBitSize); err == nil &&
			v > float64(constants.DefaultInitValue) {
			qps = v
		}
	}
	burst := constants.DefaultDiscoveryK8sClientBurst
	if raw := strings.TrimSpace(os.Getenv(constants.EnvDiscoveryK8sClientBurst)); raw != constants.EmptyString {
		if v, err := strconv.Atoi(raw); err == nil && v > constants.DefaultInitValue {
			burst = v
		}
	}
	k8sclient.SetRESTClientRateLimit(float32(qps), burst)
}
