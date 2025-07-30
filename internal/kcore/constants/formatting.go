package constants

const (
	MEMORY_UNIT_KI     = "Ki"
	MEMORY_UNIT_MI     = "Mi"
	MEMORY_UNIT_GI     = "Gi"
	MEMORY_UNIT_B      = "B"
	MEMORY_UNIT_KB     = "kb"
	MEMORY_UNIT_MB     = "mb"
	MEMORY_UNIT_GB     = "gb"
	CPU_UNIT_MILLICORE = "m"
)

const (
	SERVICE_HOST_PATTERN           = "http://%s.%s.svc.cluster.local:%d"
	FIELD_SELECTOR_INVOLVED_OBJECT = "involvedObject.name=%s"
)

const (
	CPU_MILLICORE_FORMAT = "%dm"
	CPU_CORE_FORMAT      = "%.2f"
	MEMORY_BYTES_FORMAT  = "%dB"
	MEMORY_UNIT_FORMAT   = "%.2f%s"
	NA_VALUE             = "N/A"
)
