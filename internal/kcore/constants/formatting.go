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
	ServiceHostPattern          = "http://%s.%s.svc.cluster.local:%d"
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
