package constants

import "time"

const (
	// Memory units
	KB = 1024
	MB = KB * 1024
	GB = MB * 1024

	// CPU formatting
	CPU_MILLICORE_THRESHOLD = 1000
	CPU_CORE_DIVISOR        = 1000.0

	// Availability check interval
	AVAILABILITY_CHECK_INTERVAL = 5 * time.Minute

	// Deployment status
	DEPLOYMENT_READY_REPLICAS = 1

	// Metrics API version
	METRICS_API_VERSION = "metrics.k8s.io/v1beta1"
)
