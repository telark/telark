package constants

import "github.com/plsyro/data/errors"

const (
	ErrRedisInitRetrying        errors.Error = "[redis] Redis not ready, retrying in %ds (elapsed: %ds)"
	ErrRedisInitMaxWaitExceeded errors.Error = "[redis] Redis did not become ready within %ds"
)
