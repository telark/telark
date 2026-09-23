package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/telark/auth/internal/constants"
	"github.com/telark/auth/internal/coordination/cleanup"
	"github.com/telark/auth/internal/tests/testutil"
	"github.com/telark/data/resources/finalizers"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

// waitForStreamLen polls until a stream reaches at least min entries or the
// deadline passes, so the assertions do not race the worker goroutines.
func waitForStreamLen(rdb *redis.Client, key string, minLen int64) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rdb.XLen(context.Background(), key).Val() >= minLen {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func enqueueJob(t *testing.T, stream *cleanup.StreamOps, resourceType, id string) {
	t.Helper()
	if err := stream.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	_, err := stream.Enqueue(context.Background(), map[string]any{
		constants.CleanupFieldResourceType: resourceType,
		constants.CleanupFieldResourceID:   id,
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
}

// A job whose reconcile keeps failing (no cluster + a zero pass deadline) is
// requeued until it exhausts its attempts, then lands in the dead-letter queue.
func TestManagerDrainsFailingJobToDLQ(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	resourceType := finalizers.ResourceTypeUsers
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), resourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	enqueueJob(t, stream, resourceType, "u1")

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	m.Start(context.Background())
	defer m.Stop()

	if !waitForStreamLen(rdb, constants.CleanupDLQStreamPrefix+resourceType, constants.DefaultIncrementValue) {
		t.Fatal("job never reached the DLQ after exhausting its attempts")
	}
}

// A nil election makes the replica the unconditional leader, so the loop starts
// its managers; the started manager drains the enqueued job to the DLQ, and the
// loop stops cleanly once its context is canceled.
func TestLeaderLoopStartsManagers(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	resourceType := finalizers.ResourceTypeUsers
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), resourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	enqueueJob(t, stream, resourceType, "u2")

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	loop := cleanup.NewLeaderLoop(nil, []*cleanup.Manager{m}, nil, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		loop.Run(ctx)
		close(done)
	}()

	drained := waitForStreamLen(rdb, constants.CleanupDLQStreamPrefix+resourceType, constants.DefaultIncrementValue)
	cancel()
	<-done

	if !drained {
		t.Fatal("leader loop did not start the manager (job never reached the DLQ)")
	}
}
