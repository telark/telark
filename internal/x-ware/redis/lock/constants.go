package lock

import "github.com/plsyro/data/errors"

const (
	// FenceKeySuffix is appended to the lock key to form the fence token key.
	FenceKeySuffix = ":fence"

	// HolderSeparator separates hostname, pid, and uuid in holder IDs.
	HolderSeparator = ":"
)

const (
	ErrNilRedisClient    errors.Error = "lock: redis client is nil"
	ErrEmptyKey          errors.Error = "lock: key must not be empty"
	ErrEmptyHolder       errors.Error = "lock: holder must not be empty"
	ErrZeroTTL           errors.Error = "lock: ttl must be greater than zero"
	ErrAcquireFailed     errors.Error = "lock: acquire failed: %v"
	ErrReleaseFailed     errors.Error = "lock: release failed: %v"
	ErrExtendFailed      errors.Error = "lock: extend failed: %v"
	ErrFenceTokenFailed  errors.Error = "lock: fence token read failed: %v"
)
