package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/constants"
	types "github.com/plsyro/kcore-pkg/metrics/types"
	"github.com/plsyro/kcore-pkg/resilience/circuitbreaker"
	"github.com/plsyro/kcore-pkg/resilience/ratelimiting"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	metricsClient *metricsclientset.Clientset
	initError     error
	once          sync.Once
	mu            sync.RWMutex
)

func InitMetricsClient() (*types.MetricsClient, error) {
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

func initMetricsClientOnce() (*types.MetricsClient, error) {
	once.Do(func() {
		metricsClient, initError = createMetricsClient()
		if initError != nil {
			types.Logger.Warning(string(constants.InfoMetricsAPIUnavailable))
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
		return nil, fmt.Errorf(string(constants.ErrFailedToCreateMetricsClient), err)
	}

	return metricsClient, nil
}

func createMetricsClientInstance() *types.MetricsClient {
	return &types.MetricsClient{
		Client:        metricsClient,
		Available:     false,
		LastCheck:     time.Time{},
		CheckInterval: constants.AvailabilityCheckInterval,
		RateLimiter:   ratelimiting.NewRateLimiter(constants.MetricsAPIRateLimit),
		CircuitBreaker: circuitbreaker.NewCircuitBreaker(
			constants.MetricsCircuitBreakerMaxFailures,
			constants.MetricsCircuitBreakerTimeout,
			constants.MetricsCircuitBreakerResetTimeout,
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
