package redis

import (
	"context"
	"testing"

	authredis "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

// NewRedisClientWithRetry dials the env-configured Redis and caches it; against
// an embedded server it connects on the first try and GetClient returns the
// same handle.
func TestNewRedisClientWithRetry(t *testing.T) {
	testutil.RedisEnv(t)

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
