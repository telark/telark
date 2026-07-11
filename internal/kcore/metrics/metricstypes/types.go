package metricstypes

import (
	"sync"
	"time"

	"github.com/telark/data/logger"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/resilience/circuitbreaker"
	"github.com/telark/kcore/resilience/ratelimiting"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var Logger = logger.NewCustomLogger(constants.LoggerPrefixK8sMetrics)

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
		TotalCPU    string                      `json:"totalCPU"`
		TotalMemory string                      `json:"totalMemory"`
	}

	MetricsClient struct {
		Client         *metricsclientset.Clientset
		Available      bool
		LastCheck      time.Time
		CheckInterval  time.Duration
		RateLimiter    *ratelimiting.RateLimiter
		CircuitBreaker *circuitbreaker.CircuitBreaker
		Mu             sync.RWMutex
	}

	MetricsAdapter struct {
		Client *MetricsClient
	}
)
