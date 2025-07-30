package metrics

import (
	"sync"
	"time"

	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/config/circuit_breaker"
	"github.com/plsyro/kcore-pkg/config/rate_limiting"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var logger = logging.NewCustomLogger("Metrics: ")

type ContainerMetrics struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pod    string `json:"pod"`
}

type PodMetrics struct {
	PodName     string                      `json:"podName"`
	Namespace   string                      `json:"namespace"`
	Containers  map[string]ContainerMetrics `json:"containers"`
	TotalCPU    string                      `json:"totalCpu"`
	TotalMemory string                      `json:"totalMemory"`
}

type MetricsClient struct {
	client         *metricsclientset.Clientset
	isAvailable    bool
	lastCheck      time.Time
	checkInterval  time.Duration
	rateLimiter    *rate_limiting.RateLimiter
	circuitBreaker *circuit_breaker.CircuitBreaker
	mu             sync.RWMutex
}

type MetricsAdapter struct {
	client *MetricsClient
}

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
