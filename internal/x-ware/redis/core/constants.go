package core

import "github.com/plsyro/data/errors"

const (
	EnvRedisHost            = "REDIS_HOST"
	EnvRedisPort            = "REDIS_PORT"
	EnvRedisPassword        = "REDIS_PASSWORD"
	EnvRedisDB              = "REDIS_DB"
	EnvRedisReadBufferSize  = "REDIS_READ_BUFFER_SIZE"
	EnvRedisWriteBufferSize = "REDIS_WRITE_BUFFER_SIZE"
	EnvRedisPoolSize        = "REDIS_POOL_SIZE"
	EnvRedisMinIdleConns    = "REDIS_MIN_IDLE_CONNS"
	EnvRedisMaxIdleConns    = "REDIS_MAX_IDLE_CONNS"
	EnvRedisConnMaxIdleTime = "REDIS_CONN_MAX_IDLE_TIME"
	EnvRedisConnMaxLifetime = "REDIS_CONN_MAX_LIFETIME"
)

const (
	ErrRedisHostRequired = "REDIS_HOST environment variable is " +
		"required"
	ErrRedisPortRequired = "REDIS_PORT environment variable is " +
		"required"
	ErrRedisClientNotConnected errors.Error = "redis client is not connected"
	ErrFailedInitRedisClient   errors.Error = "failed to initialize Redis client: %v"
	ErrFailedCloseRedisClient  errors.Error = "failed to close Redis client: %v"
	ErrRedisExistsError        errors.Error = "redis EXISTS error for " +
		"deduplication: %v"
	ErrRedisSetError errors.Error = "redis SET error for marking event " +
		"as processed: %v"
	ErrRedisPingError errors.Error = "redis ping failed: %v"
	ErrStringFormat   errors.Error = "%s"
	// Health check error constants
	ErrRedisHealthCheckPingError          errors.Error = "health check failed - ping error: %v"
	ErrRedisHealthCheckSetError           errors.Error = "health check failed - set operation error: %v"
	ErrRedisHealthCheckDelError           errors.Error = "health check failed - del operation error: %v"
	ErrRedisHealthCheckGetClientInfoError errors.Error = "health check failed - get client info error: %v"
	ErrRedisHealthCheckGetClientIDError   errors.Error = "health check failed - get client ID error: %v"
)

const (
	DefaultHost            = "localhost"
	DefaultPort            = "6379"
	DefaultPassword        = ""
	DefaultDB              = 0
	DefaultTimeout         = 5
	DeduplicationValue     = "1"
	DeduplicationPrefix    = "dedup:"
	UnknownEventType       = "unknown"
	existsThreshold        = 0
	initialIntervalSeconds = 1
	maxIntervalSeconds     = 30
	DefaultReadBufferSize  = 524288 // 0.5MiB (524288 bytes)
	DefaultWriteBufferSize = 524288 // 0.5MiB (524288 bytes)
	DefaultPoolSize        = 10
	DefaultMinIdleConns    = 0
	DefaultMaxIdleConns    = 0
	DefaultConnMaxIdleTime = 30 // 30 minutes
	DefaultConnMaxLifetime = 0
	DefaultEmptyString     = ""
	DefaultInitValue       = 0
	healthCheckKey         = "health_check"
)
