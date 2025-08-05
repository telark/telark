package core

import "github.com/plsyro/data/errors"

const (
	EnvRedisHost     = "REDIS_HOST"
	EnvRedisPort     = "REDIS_PORT"
	EnvRedisPassword = "REDIS_PASSWORD"
	EnvRedisDB       = "REDIS_DB"
)

const (
	ErrRedisHostRequired                    = "REDIS_HOST environment variable is required"
	ErrRedisPortRequired                    = "REDIS_PORT environment variable is required"
	ErrRedisClientNotConnected errors.Error = "redis client is not connected"
	ErrFailedInitRedisClient   errors.Error = "failed to initialize Redis client: %v"
	ErrFailedCloseRedisClient  errors.Error = "failed to close Redis client: %v"
	ErrRedisExistsError        errors.Error = "redis EXISTS error for deduplication: %v"
	ErrRedisSetError           errors.Error = "redis SET error for marking event as processed: %v"
	ErrRedisPingError          errors.Error = "redis ping failed: %v"
)

const (
	DefaultHost     = "localhost"
	DefaultPort     = "6379"
	DefaultPassword = ""
	DefaultDB       = 0
	DefaultTimeout  = 5
)
