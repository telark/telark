package redis

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	dataconstants "github.com/telark/telark/internal/data/constants"
	authredis "github.com/telark/telark/services/auth/internal/helpers/redis"
)

const (
	testServiceToken = "test-service-token"
	entryKey         = "auth:test:entry"
	entryValue       = "u-test"
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

// An entry auth signed reads back for its key until it expires, and not after: a replayed old entry reads like none.
func TestSignedEntryExpires(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, testServiceToken)
	cases := []struct {
		name string
		ttl  time.Duration
		want bool
	}{
		{"live", time.Minute, true},
		{"expired", -time.Second, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			entry, err := authredis.SignEntry(entryKey, entryValue, c.ttl)
			if err != nil {
				t.Fatal(err)
			}
			value, ok := authredis.VerifyEntry(entryKey, entry)
			if ok != c.want || ok && value != entryValue {
				t.Fatalf("VerifyEntry = (%q, %v), want %v", value, ok, c.want)
			}
		})
	}
}
