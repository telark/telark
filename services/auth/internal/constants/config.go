package constants

import "time"

const (
	DefaultPort                   = "8080"
	DefaultReadHeaderTimeout      = 10 * time.Second
	DefaultReadTimeout            = 30 * time.Second
	DefaultWriteTimeout           = 60 * time.Second
	DefaultIdleTimeout            = 120 * time.Second
	DefaultShutdownTimeout        = 30 * time.Second
	DefaultChallengeTimeout       = 60 // seconds
	DefaultSessionExpiry          = 24 // hours
	HeaderSessionToken            = "X-Session-Token"
	HeaderUserID                  = "X-User-ID"
	HeaderCredentialID            = "X-Credential-ID"
	HeaderDeviceName              = "X-Device-Name"
	HeaderDeviceType              = "X-Device-Type"
	HeaderUsername                = "X-Username"
	HeaderEmail                   = "X-Email"
	HeaderContentType             = "Content-Type"
	ContentTypeJSON               = "application/json"
	EmptyString                   = ""
	ColonSeparator                = ":"
	UnderscoreSeparator           = "_"
	DefaultQuitChannelSize        = 1
	DefaultInitValue              = 0
	DefaultIncrementValue         = 1
	DefaultColonSeparatorLength   = 1
	PanicRecoveryDelay            = 5 * time.Second
	HTTPBadRequest                = 400
	InitialCapacity               = 0
	ExitCodeError                 = 1
	AuthDataMinLengthForCredIDLen = 55
	BackupEligibleFlag            = 0x08
	BackupStateFlag               = 0x10
	AuthDataOffsetFlags           = 32
	AuthDataOffsetSignCount       = 33
	AuthDataOffsetAAGUID          = 37
	AuthDataOffsetCredIDLen       = 53
	AuthDataOffsetCredID          = 55
	AAGUIDLength                  = 16
	AttestationFormatNone         = "none"
	AttStmtKey                    = "attStmt"
	SignCountShift24              = 24
	SignCountShift16              = 16
	SignCountShift8               = 8
	MaxUpdateFields               = 2
	MaxPanicRecoveryAttempts      = 5
	MaxUserHandleLength           = 64
	RandomBytesLength             = 8
	UserHandleFormat              = "%s:%s"
	OIDCJWKSRefreshInterval       = 6 * time.Hour
	OIDCJWKSMinRefreshInterval    = 5 * time.Minute
	OIDCJWKSFetchTimeout          = 10 * time.Second
	EnvBootstrapAdmins            = "BOOTSTRAP_ADMINS"
	EnvSelfRegistrationEnabled    = "SELF_REGISTRATION_ENABLED"
	EnvReplicaID                  = "HOSTNAME"
	StandaloneReplicaID           = "standalone"

	BuiltInRoleAdmin       = "r-00000-0000-0001"
	BuiltInRoleOwner       = "r-00000-0000-0002"
	BuiltInRoleContributor = "r-00000-0000-0003"
	BuiltInRoleReadOnly    = "r-00000-0000-0004"

	// Redis config env vars
	EnvRedisRetryIntervalSec     = "REDIS_RETRY_INTERVAL_SEC"
	EnvRedisMaxWaitSec           = "REDIS_MAX_WAIT_SEC"
	EnvRedisPingTimeoutSec       = "REDIS_PING_TIMEOUT_SEC"
	DefaultRedisRetryIntervalSec = 2
	DefaultRedisMaxWaitSec       = 30
	DefaultRedisPingTimeoutSec   = 3

	// Redis key prefixes
	RedisKeyPrefixChallenge = "auth:webauthn:challenge:"
	RedisKeyPrefixNonce     = "auth:oidc:nonce:"
	RedisKeyJWKS            = "auth:oidc:jwks:google"
	RedisKeyJWKSLock        = "auth:oidc:jwks:refresh-lock"

	// Redis TTLs
	RedisTTLChallenge        = 60  // seconds — matches WebAuthn ceremony timeout
	RedisTTLNonce            = 300 // seconds — 5 minutes for OIDC flow
	RedisTTLJWKS             = 6   // hours
	OIDCNonceByteLen         = 32
	RedisTTLJWKSLock         = 10 // seconds
	RedisAsyncWorkerPoolSize = 32
	RedisAsyncWorkerTimeout  = 5 * time.Second
	RedisAsyncDrainTimeout   = 5 * time.Second
	RedisChallengeOpTimeout  = 3 * time.Second

	// Username generation (CRD regex: ^[a-zA-Z0-9_-]+$)
	UsernameMaxLocalLen  = 43
	UsernameRandomBytes  = 3
	UsernameMinLen       = 3
	EmailSplitParts      = 2
	UsernameInvalidChars = `[^a-zA-Z0-9_-]`

	// Cleanup controllers + queue
	EnvReconcileTickSeconds          = "RECONCILE_TICK_SECONDS"
	EnvReconcilePassDeadlineSeconds  = "RECONCILE_PASS_DEADLINE_SECONDS"
	EnvCleanupWorkersPerType         = "CLEANUP_WORKERS_PER_TYPE"
	EnvCleanupStreamMaxLen           = "CLEANUP_STREAM_MAXLEN"
	EnvCleanupLagAlertThreshold      = "CLEANUP_LAG_ALERT_THRESHOLD"
	EnvCleanupSweeperIntervalSeconds = "CLEANUP_SWEEPER_INTERVAL_SECONDS"
	EnvCleanupJobMaxAttempts         = "CLEANUP_JOB_MAX_ATTEMPTS"
	EnvCleanupDedupTTLSeconds        = "CLEANUP_DEDUP_TTL_SECONDS"
	EnvCleanupXClaimMinIdleSeconds   = "CLEANUP_XCLAIM_MIN_IDLE_SECONDS"
	EnvCleanupListTimeoutSeconds     = "CLEANUP_LIST_TIMEOUT_SECONDS"
	EnvCleanupPatchTimeoutSeconds    = "CLEANUP_PATCH_TIMEOUT_SECONDS"
	EnvCleanupMaxConcurrentPatches   = "CLEANUP_MAX_CONCURRENT_PATCHES"
	EnvCleanupBackoffInitialSeconds  = "RECONCILE_BACKOFF_INITIAL_SECONDS"
	EnvCleanupBackoffMaxSeconds      = "RECONCILE_BACKOFF_MAX_SECONDS"
	EnvBackfillFinalizersEnabled     = "BACKFILL_FINALIZERS_ENABLED"
	EnvBackfillBatchSize             = "BACKFILL_BATCH_SIZE"
	EnvBackfillBatchPauseMS          = "BACKFILL_BATCH_PAUSE_MS"

	DefaultReconcileTickSeconds          = 5
	DefaultReconcilePassDeadlineSeconds  = 30
	DefaultCleanupWorkersPerType         = 2
	DefaultCleanupStreamMaxLen           = 10000
	DefaultCleanupLagAlertThreshold      = 500
	DefaultCleanupSweeperIntervalSeconds = 60
	DefaultCleanupJobMaxAttempts         = 5
	DefaultCleanupDedupTTLSeconds        = 600
	DefaultCleanupXClaimMinIdleSeconds   = 60
	DefaultCleanupListTimeoutSeconds     = 10
	DefaultCleanupPatchTimeoutSeconds    = 5
	DefaultCleanupMaxConcurrentPatches   = 4
	DefaultCleanupBackoffInitialSeconds  = 5
	DefaultCleanupBackoffMaxSeconds      = 300
	DefaultBackfillBatchSize             = 10
	DefaultBackfillBatchPauseMS          = 100

	CleanupStreamPrefix    = "auth:cleanup:"
	CleanupDLQStreamPrefix = "auth:cleanup:dlq:"
	CleanupDedupKeyPrefix  = "auth:cleanup:dedup:"
	CleanupConsumerGroup   = "auth-cleanup"
	CleanupConsumerName    = "auth-cleanup-worker"

	CleanupFieldJobID         = "jobId"
	CleanupFieldResourceType  = "resourceType"
	CleanupFieldResourceID    = "resourceID"
	CleanupFieldEnqueuedAt    = "enqueuedAt"
	CleanupFieldRequestedBy   = "requestedBy"
	CleanupFieldAttempts      = "attempts"
	CleanupRequestedBySweeper = "sweeper"

	SpecFieldAssignedUsersIDs  = "assignedUsersIDs"
	SpecFieldAssignedRolesIDs  = "assignedRolesIDs"
	SpecFieldAssignedGroupsIDs = "assignedGroupsIDs"
)
