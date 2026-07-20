package authzcache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/exporter/internal/authz"
	exprdb "github.com/telark/exporter/internal/redis"
)

func TestGenerationAndForgetWithClient(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	// Fire-and-forget cache maintenance must run cleanly against a live client.
	authz.BumpGeneration(ctx)
	authz.BumpGeneration(ctx)
	authz.ForgetUserGrants(ctx, "u1")
	authz.ForgetSession(ctx, "token-1")

	if len(mr.Keys()) == 0 {
		t.Error("BumpGeneration should have written a generation key")
	}
}

func TestCacheMaintenanceWithoutClient(t *testing.T) {
	exprdb.Set(nil)
	ctx := context.Background()
	// With no client configured these must be safe no-ops, never a panic.
	authz.BumpGeneration(ctx)
	authz.ForgetUserGrants(ctx, "u1")
	authz.ForgetSession(ctx, "token-1")
}

func TestResolverDegradedWithoutBackend(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	resolver := authz.NewResolver()
	// Cache misses fall through to the session/grants backend, which is absent,
	// so both resolve to an error rather than a false positive.
	if _, err := resolver.UserIDForToken("unknown-token"); err == nil {
		t.Error("UserIDForToken should fail when the session backend is unavailable")
	}
	if _, err := resolver.GrantsForUser("u1"); err == nil {
		t.Error("GrantsForUser should fail when the grants backend is unavailable")
	}
}
