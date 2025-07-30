package types

import (
	"sync"
	"time"

	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/circuit_breaker"
	"github.com/plsyro/kcore-pkg/resilience/rate_limiting"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var Logger = logging.NewCustomLogger(constants.LOGGER_PREFIX_METRICS)

type (
	ContainerMetrics struct {
		CPU    string `json:"cpu"`
		Memory string `json:"memory"`
		Pod    string `json:"pod"`
	}

	PodMetrics struct {
		PodName     string                      `json:"podName"`
		Namespace   string                      `json:"namespace"`
		Containers  map[string]ContainerMetrics `json:"containers"`
		TotalCPU    string                      `json:"totalCpu"`
		TotalMemory string                      `json:"totalMemory"`
	}

	MetricsClient struct {
		Client         *metricsclientset.Clientset
		Available      bool
		LastCheck      time.Time
		CheckInterval  time.Duration
		RateLimiter    *rate_limiting.RateLimiter
		CircuitBreaker *circuit_breaker.CircuitBreaker
		Mu             sync.RWMutex
	}

	MetricsAdapter struct {
		Client *MetricsClient
	}
)
