package redis

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"

	authredis "github.com/telark/telark/services/auth/internal/helpers/redis"
)

// The client is cached for the process, so the server it binds to outlives every run (-count).
func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	m.Run()
	mr.Close()
}

// NewRedisClientWithRetry dials the env-configured Redis and caches it; against
// an embedded server it connects on the first try and GetClient returns the
// same handle.
func TestNewRedisClientWithRetry(t *testing.T) {
	c := authredis.NewRedisClientWithRetry(context.Background())
	if c == nil {
		t.Fatal("NewRedisClientWithRetry returned nil")
	}
	if err := c.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("ping embedded redis: %v", err)
	}
	if authredis.GetClient() != c {
		t.Fatal("GetClient did not return the cached client")
	}
}
