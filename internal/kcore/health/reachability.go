package health

import (
	"context"
	"sync"
	"time"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
)

type reachabilityState struct {
	mu        sync.Mutex
	checkedAt time.Time
	healthy   bool
}

var globalReachability = &reachabilityState{}

// IsK8sReachable reports whether the apiserver answered a cheap ServerVersion
// call within `timeout`. Results are cached for `ttl` per process so high-rate
// readiness probes do not add apiserver load. Pass zero values to use the
// kcore defaults (constants.K8sReachabilityProbeTimeout, .K8sReachabilityCacheTTL).
func IsK8sReachable(ctx context.Context, ttl time.Duration, timeout time.Duration) bool {
	if ttl <= constants.ZeroValue {
		ttl = constants.K8sReachabilityCacheTTL
	}
	if timeout <= constants.ZeroValue {
		timeout = constants.K8sReachabilityProbeTimeout
	}
	globalReachability.mu.Lock()
	if !globalReachability.checkedAt.IsZero() && time.Since(globalReachability.checkedAt) < ttl {
		healthy := globalReachability.healthy
		globalReachability.mu.Unlock()
		return healthy
	}
	globalReachability.mu.Unlock()

	healthy := probe(ctx, timeout)
	globalReachability.mu.Lock()
	globalReachability.checkedAt = time.Now()
	globalReachability.healthy = healthy
	globalReachability.mu.Unlock()
	return healthy
}

func probe(ctx context.Context, timeout time.Duration) bool {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil || client == nil {
		return false
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	done := make(chan error, constants.K8sReachabilityProbeChannelDepth)
	go func() {
		_, vErr := client.Discovery().ServerVersion()
		done <- vErr
	}()
	select {
	case <-probeCtx.Done():
		return false
	case err := <-done:
		return err == nil
	}
}
