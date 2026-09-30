package coordination

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/resources/finalizers"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
	"github.com/telark/telark/services/auth/internal/coordination/cleanup"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	testResourceType    = "user"
	testReplicaID       = "replica-1"
	testWorkersPerType  = 1
	testStreamMaxLen    = 100
	testJobMaxAttempts  = 3
	testSweeperInterval = 10 * time.Millisecond
	testCallTimeout     = 20 * time.Millisecond
)

func fastConfig() config.CleanupConfig {
	return config.CleanupConfig{
		WorkersPerType:  testWorkersPerType,
		StreamMaxLen:    testStreamMaxLen,
		SweeperInterval: testSweeperInterval,
		DedupTTL:        time.Minute,
		XClaimMinIdle:   time.Minute,
		ListTimeout:     testCallTimeout,
		PatchTimeout:    testCallTimeout,
		JobMaxAttempts:  testJobMaxAttempts,
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
	election := xwareredis.NewElectionClient(rdb, testReplicaID, "cleanup:leader", time.Minute)

	sys, err := cleanup.Bootstrap(context.Background(), rdb, cfg, newReconciler(cfg), election, testReplicaID, 10*time.Millisecond)
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
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), testResourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	if err := stream.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	m := cleanup.NewManager(cfg, testResourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)

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
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), testResourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	ingress := cleanup.NewIngress(map[string]*cleanup.StreamOps{testResourceType: stream}, cleanup.NewDedup(rdb, cfg.DedupTTL))
	sweeper := cleanup.NewSweeper(cfg, testResourceType, ingress)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx) // synchronous; returns when ctx expires
}

// A live resource that lost its cleanup finalizer (the uninstall hook strips them)
// gets it back from the sweeper; one that still has it, or is being deleted, is left alone.
func TestSweeperRestoresMissingFinalizer(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	cfg.ListTimeout, cfg.PatchTimeout = time.Second, time.Second
	deleting := "2026-01-01T00:00:00Z"
	views := []resourcesshared.CleanupView{
		{Name: "stripped"},
		{Name: "kept", Finalizers: []string{finalizers.UserCleanup}},
		{Name: "gone", DeletionTimestamp: &deleting},
	}
	var mu sync.Mutex
	var restored []string
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": views}})
			return
		}
		mu.Lock()
		restored = append(restored, r.URL.Path)
		mu.Unlock()
		_, _ = w.Write([]byte(`{"status":200}`))
	}))
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), finalizers.ResourceTypeUsers, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	ingress := cleanup.NewIngress(map[string]*cleanup.StreamOps{finalizers.ResourceTypeUsers: stream}, cleanup.NewDedup(rdb, cfg.DedupTTL))
	sweeper := cleanup.NewSweeper(cfg, finalizers.ResourceTypeUsers, ingress)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	sweeper.Run(ctx) // several ticks; the stub never changes, so each tick restores again

	mu.Lock()
	defer mu.Unlock()
	if len(restored) == constants.DefaultInitValue {
		t.Fatal("no finalizer restored")
	}
	for _, path := range restored {
		if !strings.Contains(path, "stripped") {
			t.Fatalf("restored %v, want only the stripped resource", restored)
		}
	}
}
