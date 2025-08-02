package core

import "github.com/plsyro/data-pkg/errors"

const (
	ENV_REDIS_HOST     = "REDIS_HOST"
	ENV_REDIS_PORT     = "REDIS_PORT"
	ENV_REDIS_PASSWORD = "REDIS_PASSWORD"
	ENV_REDIS_DB       = "REDIS_DB"
)

const (
	ERROR_REDIS_HOST_REQUIRED                     = "REDIS_HOST environment variable is required"
	ERROR_REDIS_PORT_REQUIRED                     = "REDIS_PORT environment variable is required"
	ERROR_REDIS_CLIENT_NOT_CONNECTED errors.Error = "redis client is not connected"
	ERROR_FAILED_INIT_REDIS_CLIENT   errors.Error = "failed to initialize Redis client: %v"
	ERROR_FAILED_CLOSE_REDIS_CLIENT  errors.Error = "failed to close Redis client: %v"
	ERROR_REDIS_EXISTS_ERROR         errors.Error = "redis EXISTS error for deduplication: %v"
	ERROR_REDIS_SET_ERROR            errors.Error = "redis SET error for marking event as processed: %v"
	ERROR_REDIS_PING_ERROR           errors.Error = "redis ping failed: %v"
)

const (
	DEFAULT_HOST     = "localhost"
	DEFAULT_PORT     = "6379"
	DEFAULT_PASSWORD = ""
	DEFAULT_DB       = 0
	DefaultTimeout   = 5
)
