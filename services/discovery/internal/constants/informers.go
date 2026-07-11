package constants

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
	KeyPrefixCoalesceBuffer                  = "coalesce:buf:"
	InformerNsGVRDelim                       = "\x00"
	MaxInformerResyncJitterFraction          = 0.5
)

var InformerExcludedKinds = []string{
	"Secret",
	"Event",
	"EndpointSlice",
	"Endpoints",
}
