package constants

import "time"

const (
	EnvDiscoveryInformerResyncSec            = "DISCOVERY_INFORMER_RESYNC_SEC"
	EnvDiscoveryInformerCoalescingWindowSec  = "DISCOVERY_INFORMER_COALESCING_WINDOW_SEC"
	EnvDiscoveryInformerCoalescingMaxWaitSec = "DISCOVERY_INFORMER_COALESCING_MAX_WAIT_SEC"
	EnvDiscoveryCoalesceBufferMaxEntries     = "DISCOVERY_COALESCE_BUFFER_MAX_ENTRIES"
	EnvDiscoveryInformerFlushRatePerSec      = "DISCOVERY_INFORMER_FLUSH_RATE_PER_SEC"
	EnvDiscoveryInformerResyncJitterFraction = "DISCOVERY_INFORMER_RESYNC_JITTER_FRACTION"
	DefaultInformerResyncSec                 = 30
	DefaultCoalesceWindowSec                 = 2
	DefaultCoalesceMaxWaitSec                = 10
	DefaultCoalesceBufferMaxEntries          = 2000
	DefaultInformerFlushRatePerSec           = 20
	DefaultInformerResyncJitterFraction      = 0.2
	TelarkConfigExcludedPollSec              = 1
	InformerSlowHandlerThreshold             = 500 * time.Millisecond
	InformerEventAdd                         = "add"
	InformerEventUpdate                      = "update"
	InformerEventDelete                      = "delete"
	// A burst of ADD events (initial replay, a new app rolling out) prewarms once.
	InformerPrewarmDebounce   = 2 * time.Second
	InformersGVRRetryInterval = 10 * time.Second
	// An ADD older than this is a replay (startup, resync), already in the stored app.
	InformerAddFreshWindow = time.Minute
	// Apps the prewarm tick creates after startup become informer-tracked on the next refresh.
	InformerKnownAppsRefresh = 30 * time.Second
	// Apps whose recorded baseline is rebuilt from their newest snapshot per tick;
	// each costs a stored GET through the flush limiter plus a manifest GET per namespace.
	InformerReconcileBackfillPerTick = 100
	KeyPrefixCoalesceBuffer          = "coalesce:buf:"
	// Last generation this leader published per app; a stored copy behind it is stale.
	KeyPrefixHistoryFloor = "history:floor:"
	HistoryFloorTTL       = 10 * time.Minute
	// Per resource, the fingerprint of the object the last flush diffed against;
	// a pre-image whose live object still matches it is already in the history.
	// The reconcile tick refreshes the TTL, so only an app no leader has seen for
	// a day loses its baseline (and a change landing while unobserved with it).
	KeyPrefixHistoryRecorded = "history:recorded:"
	HistoryRecordedTTL       = 24 * time.Hour
	// The compared roots of each resource the last flush changed (the newest
	// snapshot holds them one change earlier); shares the recorded TTL and purge.
	KeyPrefixHistoryPost = "history:post:"
	// Field of the post hash naming the generation whose flush wrote it: a newer
	// generation without one (a rollback, a flush older than this field) means
	// the newest snapshot is one change behind for resources nobody can name.
	HistoryPostGenerationField = "generation"
	HistoryInitialGeneration   = 1
	// Pre-image set written for a generation the store has not recorded yet; a
	// retried flush reuses or reclaims it instead of writing a second set.
	KeyPrefixSnapshotPending = "snap:pending:"
	SnapshotPendingTTL       = 10 * time.Minute
	// A stale stored copy is waiting on the notifier to apply the last publish;
	// re-reading it every coalesce window only adds GET load to the exporter.
	InformerFlushStaleRetry = 15 * time.Second
	// Retried flushes back off per app (base × 2^attempt, jittered) so a storm of
	// dirty apps drains instead of re-hitting an overloaded exporter every window.
	InformerFlushMaxRetryDelay       = 5 * time.Minute
	InformerFlushRetryJitterFraction = 0.2
	// Outlives a pod restart so a captured oldObject is flushed by the next leader.
	CoalesceBufferPersistTTL = 5 * time.Minute
	// Pre-images of a flush that found no inputs (every workload deleted), kept for the recreate
	// past one window and across leaders; short-lived, so a mass teardown doesn't pin them in Redis.
	KeyPrefixCoalesceHeld = "coalesce:held:"
	CoalesceHeldTTL       = 5 * time.Minute
	// A change the tick sees without an informer pre-image (a resource that joined while
	// discovery was down) waits this many consecutive ticks for the flush before it is
	// recorded against the live state; the value is "<fingerprint>:<ticks>".
	KeyPrefixHistoryDeferred        = "history:deferred:"
	HistoryDeferredTTL              = 10 * time.Minute
	HistoryDeferredMaxTicks         = 2
	InformerNsGVRDelim              = "\x00"
	InformerAppIndex                = "app"
	MaxInformerResyncJitterFraction = 0.5
)

var InformerExcludedKinds = []string{
	"Event",
	"EndpointSlice",
	"Endpoints",
}

// ComputeHealth reads exactly these status counters; an update leaving them
// and the diff roots untouched is a controller status write nothing publishes.
var InformerHealthStatusFields = []string{
	"readyReplicas",
	"desiredNumberScheduled",
	"numberReady",
	"active",
	"succeeded",
	"failed",
}
