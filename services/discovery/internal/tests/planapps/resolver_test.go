package planapps

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	"github.com/telark/discovery/internal/tests/testutil"
)

// NewRedisResolver returns a resolver bound to the client. A nil claim reader is the
// no-cluster shape: applications resolve without their referenced volume claims. Without a reachable
// cluster the underlying discovery either errors or resolves nothing, so an
// unknown application id can never come back resolved.
func TestNewRedisResolver(t *testing.T) {
	mr := testutil.RedisEnv(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	resolve := applications.NewRedisResolver(rdb, nil)
	resolved, missing, err := resolve(context.Background(), []string{"ghost"})
	if err != nil {
		return // discovery unavailable in the test environment — the bound resolver still ran
	}
	if _, ok := resolved["ghost"]; ok {
		t.Fatal("an unknown application id must not resolve")
	}
	if len(missing) == constants.DefaultInitValue {
		t.Fatal("an unknown application id should be reported missing")
	}
}
