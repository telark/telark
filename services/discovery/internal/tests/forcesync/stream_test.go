package forcesync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/coordination/forcesync"
	"github.com/telark/discovery/internal/tests/testutil"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// The stream lifecycle — group creation, enqueue, grouped read, ack, reclaim and
// age-trim — runs against an in-memory Redis.
func TestStreamOpsLifecycle(t *testing.T) {
	ctx := context.Background()
	cfg := config.LoadForceSyncConfig()
	s := forcesync.NewStreamOps(xwareredis.NewStreamClient(redisClient(t)), cfg)

	_ = s.EnsureGroup(ctx)
	id, err := s.Enqueue(ctx, map[string]any{"appName": "shop"})
	if err != nil || id == "" {
		t.Fatalf("enqueue = %q, %v", id, err)
	}
	_ = s.EnsureGroup(ctx)
	if _, err := s.Read(ctx, "consumer-1"); err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := s.Ack(ctx, id); err != nil {
		t.Fatalf("ack: %v", err)
	}
	_, _ = s.Reclaim(ctx, "consumer-2")
	_ = s.TrimByAge(ctx, time.Now())
}

// A NOGROUP error is recognised; other errors and nil are not.
func TestIsNoGroupError(t *testing.T) {
	testutil.Equal(t, "nil", forcesync.IsNoGroupError(nil), false)
	testutil.Equal(t, "other", forcesync.IsNoGroupError(errors.New("boom")), false)
	testutil.Equal(t, "nogroup", forcesync.IsNoGroupError(errors.New("NOGROUP No such key or group")), true)
}

// The dedup claim is exclusive per app until released, and reports the holding
// job id to a losing claimant.
func TestDedupClaim(t *testing.T) {
	ctx := context.Background()
	d := forcesync.NewDedup(redisClient(t), time.Minute)

	isNew, _, err := d.TryClaim(ctx, "shop", "job-1")
	if err != nil || !isNew {
		t.Fatalf("first claim = %v, %v", isNew, err)
	}
	isNew2, holder, _ := d.TryClaim(ctx, "shop", "job-2")
	if isNew2 || holder != "job-1" {
		t.Fatalf("second claim = %v, holder=%q, want false/job-1", isNew2, holder)
	}
	if err := d.Release(ctx, "shop"); err != nil {
		t.Fatalf("release: %v", err)
	}
	isNew3, _, _ := d.TryClaim(ctx, "shop", "job-3")
	testutil.Equal(t, "reclaim after release", isNew3, true)
}

// The maintenance and manager constructors wire their dependencies; Stop on an
// unstarted manager is a safe no-op.
func TestManagerConstructAndStop(t *testing.T) {
	cfg := config.LoadForceSyncConfig()
	s := forcesync.NewStreamOps(xwareredis.NewStreamClient(redisClient(t)), cfg)
	d := forcesync.NewDedup(redisClient(t), time.Minute)

	_ = forcesync.NewMaintenance(cfg, s)

	executor := func(ctx context.Context, replicaID, appName string) error { return nil }
	m := forcesync.NewManager(cfg, s, d, nil, executor, "replica-1")
	m.Stop()
}
