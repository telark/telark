package client

import (
	"fmt"
	"sync"
	"time"

	"github.com/plsyro/data/errors"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/metrics/metricstypes"
	"github.com/plsyro/kcore/resilience/circuitbreaker"
	"github.com/plsyro/kcore/resilience/ratelimiting"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	metricsClient *metricsclientset.Clientset
	clientLoader  func() (*metricsclientset.Clientset, error)
	mu            sync.RWMutex
)

func InitMetricsClient() (*metricstypes.MetricsClient, error) {
	mu.RLock()
	if metricsClient != nil {
		defer mu.RUnlock()
		return createMetricsClientInstance(), nil
	}
	if _, err := clientLoader(); err != nil {
		defer mu.RUnlock()
		return nil, err
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	if metricsClient != nil {
		return createMetricsClientInstance(), nil
	}
	client, err := clientLoader()
	if err != nil {
		return nil, err
	}
	metricsClient = client

	return createMetricsClientInstance(), nil
}

func createMetricsClient() (*metricsclientset.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("%s %v", string(errors.ErrK8sCreateConfig), err)
	}

	metricsClient, err := metricsclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedToCreateMetricsClient), err)
	}

	return metricsClient, nil
}

func createMetricsClientInstance() *metricstypes.MetricsClient {
	return &metricstypes.MetricsClient{
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
	clientLoader = makeMetricsClientLoader()
}

func makeMetricsClientLoader() func() (*metricsclientset.Clientset, error) {
	return sync.OnceValues(func() (*metricsclientset.Clientset, error) {
		client, err := createMetricsClient()
		if err != nil {
			metricstypes.Logger.Warn(string(constants.InfoMetricsAPIUnavailable))
			return nil, err
		}
		return client, nil
	})
}

func init() {
	clientLoader = makeMetricsClientLoader()
}
