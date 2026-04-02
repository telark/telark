package leader

import (
	"time"

	"github.com/plsyro/data/errors"
)

const (
	DefaultLeaseTTL      = 15 * time.Second
	DefaultRenewInterval = 5 * time.Second
	DefaultRetryInterval = 3 * time.Second
	DefaultLeaseKey      = "sync-manager:leader"
)

const (
	ErrNilRedisClient errors.Error = "leader: redis client is nil"
	ErrEmptyLeaseKey  errors.Error = "leader: lease key must not be empty"
)
