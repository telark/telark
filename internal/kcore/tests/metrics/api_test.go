package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/telark/telark/internal/kcore/constants"
	"github.com/telark/telark/internal/kcore/metrics/client"
	"github.com/telark/telark/internal/kcore/metrics/metricstypes"
	"github.com/telark/telark/internal/kcore/resilience/circuitbreaker"
	"github.com/telark/telark/internal/kcore/resilience/ratelimiting"
	"k8s.io/client-go/rest"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const breakerMaxFailures = 1

var errMetricsAPIDown = errors.New("metrics API down")

func metricsClient(t *testing.T, breakerOpen bool) *metricstypes.MetricsClient {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	clientset, err := metricsclientset.NewForConfig(&rest.Config{Host: server.URL})
	if err != nil {
		t.Fatalf("metrics clientset: %v", err)
	}
	breaker := circuitbreaker.NewCircuitBreaker(breakerMaxFailures, time.Minute, time.Hour)
	if breakerOpen {
		_ = breaker.Call(func() error { return errMetricsAPIDown })
	}
	return &metricstypes.MetricsClient{
		Client:         clientset,
		Available:      true,
		LastCheck:      time.Now(),
		CheckInterval:  time.Hour,
		RateLimiter:    ratelimiting.NewRateLimiter(TestRateLimitInterval),
		CircuitBreaker: breaker,
	}
}

func TestMetricsAPICallsReportOpenBreaker(t *testing.T) {
	calls := map[string]func(*metricstypes.MetricsClient) error{
		"get pod": func(mc *metricstypes.MetricsClient) error {
			_, err := client.GetPodMetrics(mc, TestNamespace, TestPodName)
			return err
		},
		"list pods": func(mc *metricstypes.MetricsClient) error {
			_, err := client.ListPodMetrics(mc, TestNamespace)
			return err
		},
	}
	for name, call := range calls {
		for _, open := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s, breaker open %t", name, open), func(t *testing.T) {
				err := call(metricsClient(t, open))
				if err == nil || (err.Error() == string(constants.InfoMetricsAPICircuitBreakerOpen)) != open {
					t.Fatalf("breaker open=%t: got error %v", open, err)
				}
			})
		}
	}
}
