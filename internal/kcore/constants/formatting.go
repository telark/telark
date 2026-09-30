package constants

const (
	MemoryUnitKi     = "Ki"
	MemoryUnitMi     = "Mi"
	MemoryUnitGi     = "Gi"
	MemoryUnitB      = "B"
	MemoryUnitKB     = "kb"
	MemoryUnitMB     = "mb"
	MemoryUnitGB     = "gb"
	CPUUnitMillicore = "m"
)

const (
	// Service-to-service traffic stays inside the cluster network; TLS terminates
	// at the ingress, so the in-cluster scheme is plain HTTP by design.
	ServiceHostPattern          = "http://%s.%s.svc.cluster.local:%d" //nolint:revive // in-cluster service DNS
	FieldSelectorInvolvedObject = "involvedObject.name=%s"
	MetricsAPIVersion           = "metrics.k8s.io/v1beta1"
)

const (
	CPUMillicoreFormat = "%dm"
	CPUCoreFormat      = "%.2f"
	MemoryBytesFormat  = "%dB"
	MemoryUnitFormat   = "%.2f%s"
	NAValue            = "N/A"
	ErrorFormatString  = "%s"
	Base10             = 10
	Base64             = 64
)
