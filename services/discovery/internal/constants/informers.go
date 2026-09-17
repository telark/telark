package constants

import "time"

const (
	EnvDiscoveryInformerResyncSec            = "DISCOVERY_INFORMER_RESYNC_SEC"
	EnvDiscoveryInformerCoalescingWindowSec  = "DISCOVERY_INFORMER_COALESCING_WINDOW_SEC"
	EnvDiscoveryInformerCoalescingMaxWaitSec = "DISCOVERY_INFORMER_COALESCING_MAX_WAIT_SEC"
	EnvDiscoveryCoalesceBufferMaxEntries     = "DISCOVERY_COALESCE_BUFFER_MAX_ENTRIES"
	EnvDiscoveryInformerResyncJitterFraction = "DISCOVERY_INFORMER_RESYNC_JITTER_FRACTION"
	DefaultInformerResyncSec                 = 30
	DefaultCoalesceWindowSec                 = 2
	DefaultCoalesceMaxWaitSec                = 10
	DefaultCoalesceBufferMaxEntries          = 2000
	DefaultInformerResyncJitterFraction      = 0.2
	GlobalConfigExcludedPollSec              = 1
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
	KeyPrefixCoalesceBuffer  = "coalesce:buf:"
	// Last generation this leader published per app; a stored copy behind it is stale.
	KeyPrefixHistoryFloor = "history:floor:"
	HistoryFloorTTL       = 10 * time.Minute
	// A stale stored copy is waiting on the notifier to apply the last publish;
	// re-reading it every coalesce window only adds GET load to the exporter.
	// ponytail: fixed delay, exponential backoff if apply lag routinely exceeds it.
	InformerFlushStaleRetry = 15 * time.Second
	// Outlives a pod restart so a captured oldObject is flushed by the next leader.
	CoalesceBufferPersistTTL        = 5 * time.Minute
	InformerNsGVRDelim              = "\x00"
	MaxInformerResyncJitterFraction = 0.5
)

var InformerExcludedKinds = []string{
	"Event",
	"EndpointSlice",
	"Endpoints",
}
