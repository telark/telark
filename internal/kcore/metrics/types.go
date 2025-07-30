package metrics

import (
	"sync"
	"time"

	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/resilience/circuit_breaker"
	"github.com/plsyro/kcore-pkg/resilience/rate_limiting"
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

// Constants moved to constants/metrics.go
