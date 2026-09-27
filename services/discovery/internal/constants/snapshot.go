package constants

const (
	EnvDiscoverySnapshotFetchTimeoutMs = "DISCOVERY_SNAPSHOT_FETCH_TIMEOUT_MS"

	// Covers kcore's manifest retry-on-transient (3 steps, backoff up to ~1.4s) plus one final
	// K8s GET; raise it if apiserver tail latency exceeds 3s on the target cluster.
	DefaultSnapshotFetchTimeoutMs = 5000
	// Held below the K8s rate-limiter budget so the fan-out does not starve other clients.
	SnapshotItemFetchConcurrency = 10
)
