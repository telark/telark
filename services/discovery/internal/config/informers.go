package config

import (
	"time"

	"github.com/telark/telark/services/discovery/internal/constants"
)

func InformerResyncSec() int {
	return envInt(constants.EnvDiscoveryInformerResyncSec, constants.DefaultInformerResyncSec)
}

// A long interval keeps the resync from stampeding K8s with one call per cached CR; watch
// events still flow on actual changes regardless of it.
func RollbackInformerResync() time.Duration {
	secs := envInt(constants.EnvDiscoveryRollbackInformerResync, constants.DefaultRollbackInformerResyncSec)
	return time.Duration(secs) * time.Second
}

func RollbackWorkers() int {
	return envInt(constants.EnvDiscoveryRollbackWorkers, constants.DefaultRollbackWorkers)
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

func InformerFlushRatePerSec() int {
	return envInt(constants.EnvDiscoveryInformerFlushRatePerSec, constants.DefaultInformerFlushRatePerSec)
}

// ±fraction on per-replica resync periods so replicas do not stampede apiserver LISTs in the
// same window. Clamped to [0, 0.5] downstream.
func InformerResyncJitterFraction() float64 {
	v := envFloat(constants.EnvDiscoveryInformerResyncJitterFraction, constants.DefaultInformerResyncJitterFraction)
	if v > constants.MaxInformerResyncJitterFraction {
		return constants.MaxInformerResyncJitterFraction
	}
	return v
}

// Env-tunable so production can absorb K8s API tail latency without recompilation.
func SnapshotFetchTimeout() time.Duration {
	ms := envInt(constants.EnvDiscoverySnapshotFetchTimeoutMs, constants.DefaultSnapshotFetchTimeoutMs)
	return time.Duration(ms) * time.Millisecond
}
