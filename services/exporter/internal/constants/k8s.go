package constants

const (
	EnvExporterK8sClientQPS       = "EXPORTER_K8S_CLIENT_QPS"
	EnvExporterK8sClientBurst     = "EXPORTER_K8S_CLIENT_BURST"
	DefaultExporterK8sClientQPS   = 50
	DefaultExporterK8sClientBurst = 100
	Float64ParseBitSize           = 32
)
