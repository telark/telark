package constants

const (
	EnvDiscoverySnapshotFetchTimeoutMs = "DISCOVERY_SNAPSHOT_FETCH_TIMEOUT_MS"

	// 5000ms accommodates kcore manifest retry-on-transient (3 steps, exp-backoff
	// up to ~1.4s) plus one final K8s GET. Tune higher if apiserver tail latency
	// exceeds 3s on the target cluster.
	DefaultSnapshotFetchTimeoutMs = 5000

	// SnapshotItemFetchConcurrency caps in-flight per-resource K8s GETs during
	// snapshot assembly. Held below the K8s rate-limiter budget so fan-out does
	// not starve other clients.
	SnapshotItemFetchConcurrency = 10
)
