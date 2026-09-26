package constants

import "github.com/telark/data/errors"

const (
	ErrRedisInitRetrying        errors.Error = "[redis] Redis not ready, retrying in %ds (elapsed: %ds)"
	ErrRedisInitMaxWaitExceeded errors.Error = "[redis] Redis did not become ready within %ds"
	ErrRedisCacheFlushUnscoped  errors.Error = "[redis] cache flush refused: no key prefix set"
	RedisKeyWildcard                         = "*"
)
