// Package testutil holds shared, generic helpers for the auth test suites so the
// tests stay DRY: an embedded Redis + env wiring, and small assertions.
package testutil

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// RedisEnv starts an embedded Redis and points REDIS_HOST/REDIS_PORT at it, so
// any code that dials Redis through x-ware (env-resolved) hits the in-memory
// server. Torn down automatically via t.Cleanup.
func RedisEnv(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	return mr
}

// RedisClient starts an embedded Redis and returns a go-redis client wired to
// it — for code that takes an explicit *redis.Client.
func RedisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

// Equal fails the test unless got == want.
func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
