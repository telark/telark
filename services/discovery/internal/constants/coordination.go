package constants

import "time"

const (
	StreamOperations                             = "streams:discovery:operations"
	ConsumerGroupName                            = "telark-discovery-consumer-group"
	StreamMsgFieldAppName                        = "appName"
	StreamMsgFieldNamespace                      = "namespace"
	StreamMsgFieldCycleID                        = "cycleID"
	StreamMsgFieldOperation                      = "operation"
	StreamMsgFieldEnqueuedAt                     = "enqueuedAt"
	StreamMsgFieldAttempts                       = "attempts"
	KeyPrefixLockApp                             = "lock:app:"
	KeyPrefixAnalyzerInflight                    = "analyzer:inflight:"
	KeyPrefixLockGen                             = "lock:gen:"
	KeyPrefixOpState                             = "ops:"
	KeyPrefixDedup                               = "dedup:"
	KeyPrefixGraceScale                          = "grace:scale:"
	KeyPrefixReplicaHB                           = "replica:"
	KeyPrefixResetCooldown                       = "reset:cooldown:"
	KeySuffixReplicaHB                           = ":heartbeat"
	KeySuffixReplicaAddr                         = ":addr"
	KeyElectionPrewarm                           = "election:prewarm"
	OperationTypePrewarm                         = "prewarm"
	StepEnqueued                                 = "enqueued"
	StepAcquiringLock                            = "acquiring_lock"
	StepProcessing                               = "processing"
	StepCompleted                                = "completed"
	StepFailed                                   = "failed"
	EnvReplicaID                                 = "HOSTNAME"
	EnvPodIP                                     = "POD_IP"
	LabelComponent                               = "app.kubernetes.io/component"
	EnvCoordinationBatchSize                     = "COORDINATION_BATCH_SIZE"
	EnvCoordinationBatchBlockSec                 = "COORDINATION_BATCH_BLOCK_SEC"
	EnvCoordinationMaxRetryAttempts              = "COORDINATION_MAX_RETRY_ATTEMPTS"
	EnvCoordinationLockTTLSec                    = "COORDINATION_LOCK_TTL_SEC"
	EnvCoordinationLockHeartbeatSec              = "COORDINATION_LOCK_HEARTBEAT_SEC"
	EnvCoordinationElectionTTLSec                = "COORDINATION_ELECTION_TTL_SEC"
	EnvCoordinationElectionRenewSec              = "COORDINATION_ELECTION_RENEW_SEC"
	EnvCoordinationDedupTTLSec                   = "COORDINATION_DEDUP_TTL_SEC"
	EnvCoordinationStaleClaimMinIdleSec          = "COORDINATION_STALE_CLAIM_MIN_IDLE_SEC"
	EnvCoordinationStaleClaimIntervalSec         = "COORDINATION_STALE_CLAIM_INTERVAL_SEC"
	EnvCoordinationShutdownCleanupTimeoutSec     = "COORDINATION_SHUTDOWN_CLEANUP_TIMEOUT_SEC"
	CoordinationDefaultShutdownCleanupTimeoutSec = 20
	CoordinationElectionResignTimeoutSec         = 5
	CoordinationBackgroundProcessingSlots        = 4
	DiscoveryLeaderGatePoll                      = 500 * time.Millisecond
	// Fallback for the leader's rediscovery cycle when GlobalConfig carries no
	// fetch interval. Operators set the real value through the UI setting.
	PrewarmDefaultInterval       = 60 * time.Second
	PrewarmCycleTTL              = 24 * time.Hour
	GraceScaleTTL                = 90 * time.Second
	ResetCooldownTTL             = 60 * time.Second
	ResetLoopGuardLogAt    int64 = 3
	// Bounds the operations stream in Redis; entries older than the cap are
	// long acked and only cost memory (it reached 17k entries unbounded).
	OperationsStreamMaxLen int64 = 20000
	// In-flight jobs of the running workers do not count as a backlog.
	PrewarmBacklogTolerance int64 = 8
)

// Leader rediscovery cycle bookkeeping the UI reads (see coordination.CycleStatus).
const (
	KeyPrewarmCycle             = "prewarm:cycle"
	PrewarmCycleFieldStartedAt  = "startedAt"
	PrewarmCycleFieldFinishedAt = "finishedAt"
	PrewarmCycleFieldEnqueued   = "enqueued"
)
