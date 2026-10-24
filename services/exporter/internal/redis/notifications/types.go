package notifications

import (
	"sync"

	"github.com/redis/go-redis/v9"
)

// Hands the list changes it reads from the change stream, once per replica, to the open
// streams of that list's user on this replica.
type Hub struct {
	rdb    *redis.Client
	mu     sync.Mutex
	subs   map[string]map[chan struct{}]struct{}
	closed bool
}
