package constants

import "time"

const (
	DefaultPort                 = "8080"
	DefaultReadHeaderTimeout    = 10 * time.Second
	DefaultReadTimeout          = 30 * time.Second
	DefaultWriteTimeout         = 60 * time.Second
	DefaultIdleTimeout          = 120 * time.Second
	DefaultShutdownTimeout      = 30 * time.Second
	DefaultChallengeTimeout     = 60 // seconds
	DefaultSessionExpiry        = 24 // hours
	SessionTokenBytes           = 32
	HeaderSessionToken          = "X-Session-Token"
	HeaderUserID                = "X-User-ID"
	HeaderDeviceName            = "X-Device-Name"
	HeaderDeviceType            = "X-Device-Type"
	HeaderContentType           = "Content-Type"
	HeaderOrigin                = "Origin"
	HeaderForwardedHost         = "X-Forwarded-Host"
	HeaderForwardedProto        = "X-Forwarded-Proto"
	HeaderForwardedFor          = "X-Forwarded-For"
	HeaderRetryAfter            = "Retry-After"
	IDPathParam                 = "id"
	CredentialIDPathParam       = "credentialId"
	ContentTypeJSON             = "application/json"
	EmptyString                 = ""
	ColonSeparator              = ":"
	UnderscoreSeparator         = "_"
	CommaSeparator              = ","
	DotSeparator                = "."
	SchemeSeparator             = "://"
	SchemeHTTP                  = "http"
	SchemeHTTPS                 = "https"
	DefaultInitValue            = 0
	DefaultIncrementValue       = 1
	DefaultColonSeparatorLength = 1
	PanicRecoveryDelay          = 5 * time.Second
	HTTPBadRequest              = 400
	InitialCapacity             = 0
	ExitCodeError               = 1
	BackupEligibleFlag          = 0x08
	BackupStateFlag             = 0x10
	AuthDataOffsetFlags         = 32
	AuthDataMinLengthForFlags   = AuthDataOffsetFlags + 1
	AAGUIDLength                = 16
	AttestationFormatNone       = "none"
	WebAuthnTypeCreate          = "webauthn.create"
	MaxWebAuthnInstances        = 32
	MaxRequestBodyBytes         = 1 << 20
	MaxUpdateFields             = 2
	MaxPanicRecoveryAttempts    = 5
	MaxUserHandleLength         = 64
	RandomBytesLength           = 8
	UserHandleFormat            = "%s:%s"
	OIDCJWKSRefreshInterval     = 6 * time.Hour
	OIDCJWKSMinRefreshInterval  = 5 * time.Minute
	OIDCJWKSFetchTimeout        = 10 * time.Second
	EnvPort                     = "PORT"
	EnvRPID                     = "RP_ID"
	EnvRPName                   = "RP_NAME"
	EnvRPOrigin                 = "RP_ORIGIN"
	EnvChallengeTimeout         = "CHALLENGE_TIMEOUT"
	EnvSessionExpiry            = "SESSION_EXPIRY"
	EnvBootstrapAdmin           = "BOOTSTRAP_ADMIN"
	EnvReplicaID                = "HOSTNAME"
	EnvOIDCTrustFile            = "OIDC_TRUST_FILE"
	DefaultOIDCTrustFile        = "/etc/telark/oidc/googleJwkJson"
	StandaloneReplicaID         = "standalone"

	EnvEnrollInviteTTLSec     = "ENROLL_INVITE_TTL_SEC"
	DefaultEnrollInviteTTLSec = 3600
	TelarkConfigCacheTTL      = 5 * time.Second
	TelarkConfigFlightKey     = "telarkconfig"

	// The global limit is per route over all clients: many addresses can hold one start at its cap, not fill Redis.
	EnvRateLimitPerClient       = "RATE_LIMIT_PER_CLIENT"
	EnvRateLimitGlobal          = "RATE_LIMIT_GLOBAL"
	EnvRateLimitTrustedHops     = "RATE_LIMIT_TRUSTED_HOPS"
	DefaultRateLimitPerClient   = 30
	DefaultRateLimitGlobal      = 3000
	DefaultRateLimitTrustedHops = 0
	// As long as a challenge lives, so a client holds at most its limit of live challenges per route.
	RateLimitWindowSec = RedisTTLChallenge
	// An IPv6 client usually holds a whole /64, so the /64 counts as one client.
	RateLimitIPv6PrefixBits = 64

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
	RedisKeyPrefixInvite            = "auth:passkey:invite:"
	RedisKeyPrefixInviteOf          = "auth:passkey:invite-of:"
	RedisKeyPrefixEnrollTokensOf    = "auth:passkey:enroll-tokens-of:"
	RedisKeyPrefixPendingUser       = "auth:webauthn:pending-user:"
	RedisKeyPrefixNonce             = "auth:oidc:nonce:"
	RedisKeyPrefixBootstrapAssert   = "auth:bootstrap:assert:"
	RedisKeyPrefixFrozen            = "auth:webauthn:frozen:"
	RedisKeyPrefixRateLimit         = "auth:ratelimit:"
	CookiePath                      = "/"
	CookieMaxAgeDelete              = -1 // net/http's way of sending Max-Age=0
	RedisChallengeEntry             = "1"
	RedisMatchAll                   = "*"
	RedisScanCount                  = 1000

	// Redis TTLs
	RedisTTLChallenge      = 60  // seconds — matches WebAuthn ceremony timeout
	RedisTTLNonce          = 300 // seconds — 5 minutes for OIDC flow
	RedisTTLEnrollToken    = 600 // seconds — 10 minutes to open a user's own link on the other host, or break-glass's
	RedisTTLRecoveryFreeze = 120 // seconds, extended before each step of a recovery, so it outlives RecoveryHold
	// A create auth sent before the freeze lands by 42.2 s: no exporter attempt starts after its 30 s call (+0.2 s retry delay), one lasts 12 s.
	// An identity PATCH lands by 45 s: none starts after its route's 20 s deadline, then the exporter reads (10 s) and writes (15 s).
	RecoveryHold             = 45 * time.Second
	OIDCNonceByteLen         = 32
	RedisAsyncWorkerPoolSize = 32
	RedisAsyncWorkerTimeout  = 5 * time.Second
	RedisAsyncDrainTimeout   = 5 * time.Second
	RedisChallengeOpTimeout  = 3 * time.Second
	RedisScanTimeout         = 30 * time.Second

	// KEYS: invite-of:<user>, invite:<digest>. ARGV: invite prefix, signed entry naming the user, digest, TTL seconds.
	ScriptIssueInvite = `local previous = redis.call('GET', KEYS[1])
if previous then redis.call('DEL', ARGV[1] .. previous) end
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[4])
redis.call('SET', KEYS[1], ARGV[3], 'EX', ARGV[4])
return 1`
	// KEYS: invite-of:<user>. ARGV: invite prefix.
	ScriptRevokeInvite = `local previous = redis.call('GET', KEYS[1])
if previous then redis.call('DEL', ARGV[1] .. previous) end
return redis.call('DEL', KEYS[1])`
	// KEYS: enroll-tokens-of:<user>. ARGV: enroll-token prefix.
	ScriptDropEnrollTokens = `for _, digest in ipairs(redis.call('SMEMBERS', KEYS[1])) do redis.call('DEL', ARGV[1] .. digest) end
return redis.call('DEL', KEYS[1])`
	// KEYS: enroll-token:<digest>, invite:<digest>. ARGV: invite-of prefix, digest, the ceremony's owner.
	// Returns {entry, key}: the stored entry, verified by the caller, and the key it was spent from.
	ScriptConsumeEnrollToken = `local entry = redis.call('GETDEL', KEYS[1])
if entry then return {entry, KEYS[1]} end
entry = redis.call('GETDEL', KEYS[2])
if not entry then return false end
if redis.call('GET', ARGV[1] .. ARGV[3]) == ARGV[2] then redis.call('DEL', ARGV[1] .. ARGV[3]) end
return {entry, KEYS[2]}`

	// The approval challenge: random bytes, then the SHA-256 of the label, the request id and the action, NUL-joined.
	BootstrapChallengeRandomBytes = 32
	BootstrapChallengeLabel       = "telark-bootstrap-request-v1"
	BootstrapChallengeSeparator   = "\x00"

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
	EnvBackfillBatchSize             = "BACKFILL_BATCH_SIZE"
	EnvBackfillBatchPauseMS          = "BACKFILL_BATCH_PAUSE_MS"

	DefaultReconcileTickSeconds                = 5
	DefaultReconcilePassDeadlineSeconds        = 30
	DefaultCleanupWorkersPerType               = 2
	DefaultCleanupStreamMaxLen                 = 10000
	DefaultCleanupLagAlertThreshold            = 500
	DefaultCleanupSweeperIntervalSeconds       = 60
	DefaultCleanupJobMaxAttempts               = 5
	DefaultCleanupDedupTTLSeconds              = 600
	DefaultCleanupXClaimMinIdleSeconds         = 60
	DefaultCleanupListTimeoutSeconds           = 10
	DefaultCleanupPatchTimeoutSeconds          = 5
	DefaultCleanupMaxConcurrentPatches         = 4
	DefaultCleanupBackoffInitialSeconds        = 5
	DefaultCleanupBackoffMaxSeconds            = 300
	DefaultBackfillBatchSize                   = 10
	DefaultBackfillBatchPauseMS                = 100
	CleanupDLQMaxLen                     int64 = 1000

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

	SpecFieldUserRefs  = "userRefs"
	SpecFieldRoleRefs  = "roleRefs"
	SpecFieldGroupRefs = "groupRefs"
)
