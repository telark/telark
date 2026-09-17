package authz

import (
	"sync"
	"time"
)

// Errors are never cached: a denied or expired session must re-hit the
// exporter on the next request instead of being pinned for the TTL.
func lookup[T any](m *sync.Map, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	if raw, found := m.Load(key); found {
		if entry, isEntry := raw.(cacheEntry[T]); isEntry && time.Now().Before(entry.expiresAt) {
			return entry.value, nil
		}
	}
	value, err := load()
	if err != nil {
		return value, err
	}
	m.Store(key, cacheEntry[T]{value: value, expiresAt: time.Now().Add(ttl)})
	return value, nil
}
