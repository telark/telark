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
	SessionTokenBytes             = 32
	HeaderSessionToken            = "X-Session-Token"
	HeaderUserID                  = "X-User-ID"
	HeaderCredentialID            = "X-Credential-ID"
	HeaderDeviceName              = "X-Device-Name"
	HeaderDeviceType              = "X-Device-Type"
	HeaderContentType             = "Content-Type"
	HeaderOrigin                  = "Origin"
	HeaderForwardedHost           = "X-Forwarded-Host"
	HeaderForwardedProto          = "X-Forwarded-Proto"
	IDPathParam                   = "id"
	ContentTypeJSON               = "application/json"
	EmptyString                   = ""
	ColonSeparator                = ":"
	UnderscoreSeparator           = "_"
	CommaSeparator                = ","
	DotSeparator                  = "."
	SchemeSeparator               = "://"
	SchemeHTTP                    = "http"
	SchemeHTTPS                   = "https"
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
	AuthDataMinLengthForFlags     = AuthDataOffsetFlags + 1
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
	EnvPort                       = "PORT"
	EnvRPID                       = "RP_ID"
	EnvRPName                     = "RP_NAME"
	EnvRPOrigin                   = "RP_ORIGIN"
	EnvChallengeTimeout           = "CHALLENGE_TIMEOUT"
	EnvSessionExpiry              = "SESSION_EXPIRY"
	EnvBootstrapAdmins            = "BOOTSTRAP_ADMINS"
	EnvSelfRegistrationEnabled    = "SELF_REGISTRATION_ENABLED"
	EnvReplicaID                  = "HOSTNAME"
	StandaloneReplicaID           = "standalone"

	BuiltInRoleAdmin    = "r-00000-0000-0001"
	BuiltInRoleReadOnly = "r-00000-0000-0004"

	// Redis config env vars
	EnvRedisRetryIntervalSec     = "REDIS_RETRY_INTERVAL_SEC"
	EnvRedisMaxWaitSec           = "REDIS_MAX_WAIT_SEC"
	EnvRedisPingTimeoutSec       = "REDIS_PING_TIMEOUT_SEC"
	DefaultRedisRetryIntervalSec = 2
	DefaultRedisMaxWaitSec       = 30
	DefaultRedisPingTimeoutSec   = 3

	// Redis key prefixes
	RedisKeyPrefixChallenge         = "auth:webauthn:challenge:"
	RedisKeyPrefixRegistrationOwner = "auth:webauthn:registration-owner:"
	RedisKeyPrefixEnrolledCeremony  = "auth:webauthn:enrolled-ceremony:"
	RedisKeyPrefixEnrollToken       = "auth:passkey:enroll-token:"
	RedisKeyPrefixNonce             = "auth:oidc:nonce:"
	RedisKeyJWKS                    = "auth:oidc:jwks:google"

	// Redis TTLs
	RedisTTLChallenge        = 60  // seconds — matches WebAuthn ceremony timeout
	RedisTTLNonce            = 300 // seconds — 5 minutes for OIDC flow
	RedisTTLEnrollToken      = 600 // seconds — 10 minutes to open the enrollment link on the other host
	RedisTTLJWKS             = 6   // hours
	OIDCNonceByteLen         = 32
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
	SpaceSeparator       = " "

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
	CleanupDLQMaxLen               int64 = 1000

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
