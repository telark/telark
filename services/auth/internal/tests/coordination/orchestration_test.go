package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/coordination/cleanup"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	"github.com/telark/auth/internal/tests/testutil"
	"github.com/telark/data/resources/finalizers"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

func fastConfig() config.CleanupConfig {
	return config.CleanupConfig{
		WorkersPerType:  1,
		StreamMaxLen:    100,
		SweeperInterval: 10 * time.Millisecond,
		DedupTTL:        time.Minute,
		XClaimMinIdle:   time.Minute,
		ListTimeout:     20 * time.Millisecond,
		PatchTimeout:    20 * time.Millisecond,
		JobMaxAttempts:  3,
	}
}

func newReconciler(cfg config.CleanupConfig) *cleanupctrl.Reconciler {
	return cleanupctrl.NewReconciler(cfg, cleanupctrl.DefaultTargets(), constants.GetLogger(constants.LoggerPrefixCleanup))
}

// Bootstrap wires the whole cleanup system — a stream + consumer group per
// resource type, dedup, ingress, managers, sweepers, and the leader loop — and
// the resulting ingress accepts work.
func TestBootstrap(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	election := xwareredis.NewElectionClient(rdb, "replica-1", "cleanup:leader", time.Minute)

	sys, err := cleanup.Bootstrap(context.Background(), rdb, cfg, newReconciler(cfg), election, "replica-1", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if sys == nil || sys.Ingress == nil || sys.LeaderLoop == nil {
		t.Fatal("Bootstrap returned an incomplete system")
	}

	res, err := sys.Ingress.Enqueue(context.Background(), cleanup.EnqueueRequest{
		ResourceType: finalizers.ResourceTypeUsers, ResourceID: "u1",
	})
	if err != nil || !res.Enqueued {
		t.Fatalf("ingress enqueue = (%+v, %v)", res, err)
	}
}

// A manager starts its worker pool and stops cleanly (cancel + wait) — the
// workers block on the empty stream and exit on shutdown.
func TestManagerStartStop(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), "user", cfg.StreamMaxLen, cfg.XClaimMinIdle)
	if err := stream.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	m := cleanup.NewManager(cfg, "user", stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), "replica-1")

	m.Start(context.Background())
	time.Sleep(20 * time.Millisecond) // let the worker poll at least once
	m.Stop()                          // cancels + waits; must not hang or panic
}

// The sweeper's Run loop ticks and returns on context cancellation; each tick
// lists deletion candidates (which fail fast here with no cluster) without
// crashing the loop.
func TestSweeperRun(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), "user", cfg.StreamMaxLen, cfg.XClaimMinIdle)
	ingress := cleanup.NewIngress(map[string]*cleanup.StreamOps{"user": stream}, cleanup.NewDedup(rdb, cfg.DedupTTL))
	sweeper := cleanup.NewSweeper(cfg, "user", ingress)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx) // synchronous; returns when ctx expires
}
