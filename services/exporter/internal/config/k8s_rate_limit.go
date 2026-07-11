package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/kcore/k8sclient"
)

func ApplyKubernetesRESTRateLimit() {
	qps := float64(constants.DefaultExporterK8sClientQPS)
	if raw := strings.TrimSpace(os.Getenv(constants.EnvExporterK8sClientQPS)); raw != constants.EmptyString {
		if v, err := strconv.ParseFloat(raw, constants.Float64ParseBitSize); err == nil &&
			v > float64(constants.DefaultInitValue) {
			qps = v
		}
	}
	burst := constants.DefaultExporterK8sClientBurst
	if raw := strings.TrimSpace(os.Getenv(constants.EnvExporterK8sClientBurst)); raw != constants.EmptyString {
		if v, err := strconv.Atoi(raw); err == nil && v > constants.DefaultInitValue {
			burst = v
		}
	}
	k8sclient.SetRESTClientRateLimit(float32(qps), burst)
}
