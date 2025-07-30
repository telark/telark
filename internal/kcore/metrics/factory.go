package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/config/circuit_breaker"
	"github.com/plsyro/kcore-pkg/config/rate_limiting"
	"github.com/plsyro/kcore-pkg/constants"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	metricsClient *metricsclientset.Clientset
	initError     error
	once          sync.Once
	mu            sync.RWMutex
)

func InitMetricsClient() (*MetricsClient, error) {
	mu.RLock()
	if metricsClient != nil && initError == nil {
		defer mu.RUnlock()
		return createMetricsClientInstance(), nil
	}
	if initError != nil {
		defer mu.RUnlock()
		return nil, initError
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if metricsClient != nil && initError == nil {
		return createMetricsClientInstance(), nil
	}
	if initError != nil {
		return nil, initError
	}

	return initMetricsClientOnce()
}

func initMetricsClientOnce() (*MetricsClient, error) {
	once.Do(func() {
		metricsClient, initError = createMetricsClient()
		if initError != nil {
			logger.Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_CREATE_METRICS_CLIENT), initError))
		}
	})

	if initError != nil {
		return nil, initError
	}

	return createMetricsClientInstance(), nil
}

func createMetricsClient() (*metricsclientset.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%s %v", string(errors.ERROR_K8S_CREATE_CONFIG), err)
	}

	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ERROR_FAILED_TO_CREATE_METRICS_CLIENT), err)
	}

	return metricsClient, nil
}

func createMetricsClientInstance() *MetricsClient {
	return &MetricsClient{
		client:        metricsClient,
		isAvailable:   false,
		lastCheck:     time.Time{},
		checkInterval: AVAILABILITY_CHECK_INTERVAL,
		rateLimiter:   rate_limiting.NewRateLimiter(rate_limiting.METRICS_API_RATE_LIMIT),
		circuitBreaker: circuit_breaker.NewCircuitBreaker(
			circuit_breaker.METRICS_CIRCUIT_BREAKER_MAX_FAILURES,
			circuit_breaker.METRICS_CIRCUIT_BREAKER_TIMEOUT,
			circuit_breaker.METRICS_CIRCUIT_BREAKER_RESET_TIMEOUT,
		),
	}
}

func ResetClient() {
	mu.Lock()
	defer mu.Unlock()
	metricsClient = nil
	initError = nil
	once = sync.Once{}
}
