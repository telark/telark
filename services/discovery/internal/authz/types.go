package authz

import (
	"sync"
	"time"

	"github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/clients"
)

type clientSource struct {
	client *clients.AuthzClient
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// ponytail: unbounded memo keyed by token digest / user ID; swap for a
// size-capped LRU if token churn ever matters.
type Resolver struct {
	inner  authz.Resolver
	tokens sync.Map
	grants sync.Map
}
