// Package testutil holds shared, generic helpers for the discovery test suites.
package testutil

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// Equal fails the test unless got == want.
func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// RedisEnv starts an embedded Redis and points REDIS_HOST/REDIS_PORT at it, so
// code that dials Redis through x-ware (env-resolved) hits the in-memory server.
func RedisEnv(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	return mr
}
