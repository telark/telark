package config

import (
	"time"

	"github.com/telark/discovery/internal/constants"
)

func InformerResyncSec() int {
	return envInt(constants.EnvDiscoveryInformerResyncSec, constants.DefaultInformerResyncSec)
}

// RollbackInformerResync returns the duration used for the RollbackController's
// dynamic informer resync. A long interval (default 10 minutes) prevents the
// resync from stampeding K8s API calls for every cached CR. Watch events still
// flow on actual changes regardless of this value.
func RollbackInformerResync() time.Duration {
	secs := envInt(constants.EnvDiscoveryRollbackInformerResync, constants.DefaultRollbackInformerResyncSec)
	return time.Duration(secs) * time.Second
}

func CoalesceWindowSec() int {
	return envInt(constants.EnvDiscoveryInformerCoalescingWindowSec, constants.DefaultCoalesceWindowSec)
}

func CoalesceMaxWaitSec() int {
	return envInt(constants.EnvDiscoveryInformerCoalescingMaxWaitSec, constants.DefaultCoalesceMaxWaitSec)
}

func CoalesceBufferMaxEntries() int {
	return envInt(constants.EnvDiscoveryCoalesceBufferMaxEntries, constants.DefaultCoalesceBufferMaxEntries)
}

// InformerResyncJitterFraction returns the ±fraction applied to per-replica
// informer resync periods so multiple replicas do not stampede apiserver LISTs
// in the same window. Clamped to [0, 0.5] downstream.
func InformerResyncJitterFraction() float64 {
	v := envFloat(constants.EnvDiscoveryInformerResyncJitterFraction, constants.DefaultInformerResyncJitterFraction)
	if v > constants.MaxInformerResyncJitterFraction {
		return constants.MaxInformerResyncJitterFraction
	}
	return v
}

// SnapshotFetchTimeout bounds a single per-resource K8s GET during snapshot
// assembly. Env-tunable so production can absorb K8s API tail latency without
// recompilation.
func SnapshotFetchTimeout() time.Duration {
	ms := envInt(constants.EnvDiscoverySnapshotFetchTimeoutMs, constants.DefaultSnapshotFetchTimeoutMs)
	return time.Duration(ms) * time.Millisecond
}
