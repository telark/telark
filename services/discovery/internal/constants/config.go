package constants

import "time"

const (
	ServiceIDDiscovery = "discovery"
	MainPort           = "8080"
	DefaultReadTimeout = 90 * time.Second
	HTTPScheme         = "http"
	// Must exceed long-running handlers (e.g. Application Force Sync ~3 minutes).
	DefaultWriteTimeout             = 4 * time.Minute
	DefaultReadHeaderTimeout        = 30 * time.Second
	DefaultIdleTimeout              = 120 * time.Second
	PanicRecoveryDelay              = 5 * time.Second
	HighLoadBackoff                 = 2
	MaxRetryAttempts                = 10
	RetryBackoff                    = 1
	NatsPublishMaxRetries           = 10
	NatsPublishRetryDelay           = 500 * time.Millisecond
	NatsPublishMaxRetryDelay        = 30 * time.Second
	ClusterVersionPatchMaxAttempts  = 5
	ClusterVersionPatchRetryBackoff = 5 * time.Second
	GlobalConfigReadyRetryBackoff   = 5 * time.Second
	DefaultQuitChannelSize          = 1
	DefaultQueueSize                = 100
	DefaultAddValue                 = 1
	DefaultInitValue                = 0
	DefaultReturnValue              = -1
	ExitCodeFatal                   = 1
	EmptyString                     = ""
	NameParam                       = "name"
	FormatVerbPrefix                = "%"
	SnapshotPathKey                 = "path"
	NamespaceParam                  = "namespace"
	SelectorParam                   = "selector"
	NamespaceParamQuery             = "namespace"
	NamespaceAll                    = "ALL"
	SelectorTypeLabels              = "labels"
	SelectorTypeText                = "text"
	DefaultLockTTL                  = 30 * time.Second
	RemovalStabilizationWindow      = 45 * time.Second

	// rest
	ApplicationJSON = "application/json"
	HeaderUserID    = "X-User-ID"

	// optimization constants
	StringBuilderSize = 64
	BufferSize        = 512
	SimpleConcatLimit = 2

	// Parsing / numeric constants for linters
	IntBase10                      = 10
	IntBitSize64                   = 64
	ZeroInt64        int64         = 0
	ZeroDuration     time.Duration = 0
	StatusPending    string        = "pending"
	StatusInProgress string        = "in_progress"
	StatusSuccess    string        = "success"
	StatusFailed     string        = "failed"
	PathSeparator    string        = "/"
	DashSeparator    string        = "-"
	ColonSeparator   string        = ":"
	SpaceSeparator                 = " "
	Wildcard         string        = "*"
	TwoValue                       = 2
	ThreeValue                     = 3

	// Snapshot note (split for line-length limits)
	SnapshotNote = "Manifests reflect K8s state at snapshot time. " +
		"Due to reconciliation timing, some fields may reflect the new state rather than the " +
		"previous state. Review before applying."

	// Manifest kind ordering for snapshots (aligns with apply order)
	ManifestOrderServiceAccount          = 1
	ManifestOrderConfigMap               = 2
	ManifestOrderSecret                  = 3
	ManifestOrderPersistentVolumeClaim   = 4
	ManifestOrderService                 = 5
	ManifestOrderNetworkPolicy           = 6
	ManifestOrderDeployment              = 7
	ManifestOrderStatefulSet             = 8
	ManifestOrderDaemonSet               = 9
	ManifestOrderJob                     = 10
	ManifestOrderCronJob                 = 11
	ManifestOrderIngress                 = 12
	ManifestOrderHorizontalPodAutoscaler = 13
	ManifestOrderVerticalPodAutoscaler   = 14
	ManifestOrderUnknown                 = 99

	// Snapshot history / exporter retention (same env as exporter-service)
	DefaultSnapshotsMaxVersions = 5
	MinSnapshotsMaxVersions     = 1

	// Enrichment wait loop
	WaitTimeoutDefaultSec         = 15
	WaitTimeoutMaxSec             = 30
	DefaultSnapshotScopeDirectory = "apps"
	ApplicationResetMessage       = "application reset triggered"

	// Common
	IDPathParam         = "id"
	RollbackIDPathParam = "rollbackId"

	// ProtectionPlanDeployTimeout bounds the K8s server-side-apply phase when
	// preparing or updating a protection plan. Detached from the HTTP request
	// context so the apply is not aborted if the caller disconnects mid-flight.
	ProtectionPlanDeployTimeout = 30 * time.Second

	// ProtectionPlanLifecycleTimeout bounds Cancel / Clear / Reactivate /
	// Duplicate handlers. Destructive K8s + exporter sequences must run to
	// completion even if the HTTP caller disconnects.
	ProtectionPlanLifecycleTimeout = 30 * time.Second

	// AppResetHandlerTimeout bounds the per-app destructive reset
	// (Redis SCAN/DEL fan-out + snapshot directory removal + exporter delete).
	AppResetHandlerTimeout = 30 * time.Second

	// InsightsReadTimeout bounds the windowed cache reads behind the insights read
	// route. Short: these are a handful of Redis GETs for one page of apps.
	InsightsReadTimeout = 5 * time.Second

	// NATSConnectTimeout bounds NATS dial-with-retry for fetching the shared
	// publisher client.
	NATSConnectTimeout = 30 * time.Second
)

// Logger prefixes
const (
	LoggerPrefixDiscoveryManager   = "DiscoveryManager: "
	LoggerPrefixEventPublisher     = "EventPublisher: "
	LoggerPrefixRollbackController = "RollbackController: "
	LoggerCircuitBreaker           = "CircuitBreaker: "
	LoggerPrefixRedis              = "Redis: "
)

// Redis constants
const (
	EnvRedisRetryIntervalSec       = "REDIS_RETRY_INTERVAL_SEC"
	EnvRedisMaxWaitSec             = "REDIS_MAX_WAIT_SEC"
	EnvRedisPingTimeoutSec         = "REDIS_PING_TIMEOUT_SEC"
	DefaultRedisRetryIntervalSec   = 5
	DefaultRedisMaxWaitSec         = 300
	DefaultRedisPingTimeoutSec     = 3
	RedisDialMaxWaitForeverSeconds = 0
)

// Status endpoints
const (
	StatusReadinessEp = "status/ready"
	StatusLivenessEp  = "status/live"
)

// Circuit breaker constants
const (
	CircuitBreakerDefaultFailureThreshold = 5
	CircuitBreakerDefaultSuccessThreshold = 2
	CircuitBreakerDefaultTimeout          = 60 * time.Second
	CircuitBreakerRedisFailureThreshold   = 3
	CircuitBreakerRedisTimeout            = 30 * time.Second
	CircuitBreakerNatsFailureThreshold    = 5
	CircuitBreakerNatsTimeout             = 45 * time.Second
	CircuitBreakerRestFailureThreshold    = 5
	CircuitBreakerRestSuccessThreshold    = 2
	CircuitBreakerRestTimeout             = 30 * time.Second
	CircuitBreakerHalfOpenMaxProbes       = 1
)

// Acronym map for display name restoration
var AcronymMap = map[string]string{
	"nats": "NATS", "redis": "Redis", "api": "API", "sql": "SQL",
	"http": "HTTP", "https": "HTTPS", "grpc": "gRPC", "ui": "UI",
	"db": "DB", "tls": "TLS", "ssl": "SSL", "jwt": "JWT",
	"oauth": "OAuth", "saml": "SAML", "ldap": "LDAP", "dns": "DNS",
	"cpu": "CPU", "gpu": "GPU",
}
