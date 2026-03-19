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
	//nolint:revive // Cluster-local service addresses can be plain HTTP in this environment.
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
	EmptyString        = ""
	ErrorFormatString  = "%s"
	Base10             = 10
	Base64             = 64
	EmptySliceLength   = 0
	SingleItem         = 1
	WorkerPoolAddCount = 1
	ZeroValue          = 0
)
