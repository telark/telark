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
	RetryBackoff                    = 1
	NatsPublishMaxRetries           = 10
	NatsPublishRetryDelay           = 500 * time.Millisecond
	NatsPublishMaxRetryDelay        = 30 * time.Second
	ClusterVersionPatchMaxAttempts  = 5
	ClusterVersionPatchRetryBackoff = 5 * time.Second
	TelarkConfigReadyRetryBackoff   = 5 * time.Second
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
	NamespaceAll                    = "ALL"
	DefaultLockTTL                  = 30 * time.Second
	KeyPrefixLockPlanDecision       = "lock:plan-decision:"
	RemovalStabilizationWindow      = 45 * time.Second

	// rest
	ApplicationJSON   = "application/json"
	HeaderUserID      = "X-User-ID"
	HeaderContentType = "Content-Type"
	HeaderRetryAfter  = "Retry-After"
	HeaderETag        = "ETag"
	HeaderIfNoneMatch = "If-None-Match"

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
	DefaultSnapshotsMaxVersions   = 5
	MinSnapshotsMaxVersions       = 1
	DefaultSnapshotScopeDirectory = "apps"
	ApplicationResetMessage       = "application reset triggered"

	// Common
	IDPathParam         = "id"
	RollbackIDPathParam = "rollbackId"

	// Plan deploy and lifecycle handlers run detached from the request context: a destructive
	// K8s + exporter sequence must finish even if the HTTP caller disconnects mid-flight.
	ProtectionPlanDeployTimeout    = 30 * time.Second
	ProtectionPlanLifecycleTimeout = 30 * time.Second
	AppResetHandlerTimeout         = 30 * time.Second
	// Heartbeat-extended plan locks: the TTL only has to outlive a stalled holder, never the budget.
	PlanLockTTL               = 2 * ProtectionPlanDeployTimeout
	PlanLockHeartbeatInterval = ProtectionPlanDeployTimeout / 3
	CategoryReadTimeout       = 10 * time.Second
	// Plan and rollback bodies are small JSON documents; anything larger is refused.
	MaxRequestBodyBytes = 1 << 20
	// The forward runs inside AppResetHandlerTimeout; this bounds the leader's own answer.
	AppResetForwardTimeout = 20 * time.Second
	// A handful of Redis GETs for one page of apps.
	InsightsReadTimeout = 5 * time.Second
	// Best-effort analyzer job XADD on the publish path; a slow Redis must never stall publishing.
	InsightsTriggerTimeout = 2 * time.Second
	// The UI reads one app per call; the cap only bounds hand-built requests.
	InsightsReadMaxApps = 100
	// The publish is async through NATS and the notifier: the job waits, off the
	// publish path, until the exporter serves the entry's generation.
	InsightsEnqueueStoreWaitAttempts = 10
	InsightsEnqueueStorePollInterval = 500 * time.Millisecond
	NATSConnectTimeout               = 30 * time.Second
)

// Insights row index: a per-replica read cache behind the cluster-wide insights list.
const (
	EnvInsightsIndexRefreshSec     = "INSIGHTS_INDEX_REFRESH_SEC"
	EnvInsightsIndexResyncSec      = "INSIGHTS_INDEX_RESYNC_SEC"
	EnvInsightsStaleAfterSec       = "INSIGHTS_STALE_AFTER_SEC"
	DefaultInsightsIndexRefreshSec = 15
	DefaultInsightsIndexResyncSec  = 300
	DefaultInsightsStaleAfterSec   = 86400
	InsightsIndexBatchSize         = 200
	// Deltas re-read this far behind the newest score seen, absorbing writes that land out of order.
	InsightsIndexOverlapMs      = 5000
	InsightsIndexSyncTimeout    = 30 * time.Second
	InsightsEnvironmentsRefresh = 60 * time.Second
	InsightsIndexRetryAfterSec  = 5
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

// Protection plan report constants
const (
	DefaultReportCaptureTimeout = 10 * time.Second
	EnvReportMaxViolations      = "PROTECTION_PLAN_REPORT_MAX_VIOLATIONS"
	DefaultReportMaxViolations  = 5000
	EnvReportCheckpointSec      = "PROTECTION_PLAN_REPORT_CHECKPOINT_SEC"
	DefaultReportCheckpointSec  = 900
	ReportCheckpointMinSec      = 60
	ReportCheckpointMaxSec      = 1800
	// Half the exporter's ReportMaxBodyBytes.
	ReportRequestBudgetBytes  = 16 << 20
	KeyPrefixReportLedgerLock = "lock:reports:ledger:"
)

// Acronym map for display name restoration
var AcronymMap = map[string]string{
	"nats": "NATS", "redis": "Redis", "api": "API", "sql": "SQL",
	"http": "HTTP", "https": "HTTPS", "grpc": "gRPC", "ui": "UI",
	"db": "DB", "tls": "TLS", "ssl": "SSL", "jwt": "JWT",
	"oauth": "OAuth", "saml": "SAML", "ldap": "LDAP", "dns": "DNS",
	"cpu": "CPU", "gpu": "GPU",
}
