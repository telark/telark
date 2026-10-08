package coordination

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/telark/telark/internal/data/resources/finalizers"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/constants"
	"github.com/telark/telark/services/auth/internal/coordination/cleanup"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	testBackoffInitial = 100 * time.Millisecond
	failedReadsToWatch = 4
)

var errRedisDown = errors.New("redis down")

// Every command fails as against an unreachable Redis; each stream read is timed.
type redisDown struct{ reads chan<- time.Time }

func (redisDown) DialHook(next redis.DialHook) redis.DialHook { return next }

func (redisDown) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h redisDown) ProcessHook(_ redis.ProcessHook) redis.ProcessHook {
	return func(_ context.Context, cmd redis.Cmder) error {
		if cmd.Name() == "xreadgroup" {
			select {
			case h.reads <- time.Now():
			default:
			}
		}
		cmd.SetErr(errRedisDown)
		return errRedisDown
	}
}

func workerNameFor(replicaID string) string {
	return constants.CleanupConsumerName + constants.UnderscoreSeparator + replicaID
}

// waitForDLQ polls until the type's dead-letter stream has an entry or the
// deadline passes, so the assertions do not race the worker goroutines.
func waitForDLQ(rdb *redis.Client) bool {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rdb.XLen(context.Background(), constants.CleanupDLQStreamPrefix+finalizers.ResourceTypeUsers).Val() > constants.DefaultInitValue {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func enqueueJob(t *testing.T, stream *cleanup.StreamOps, id string) {
	t.Helper()
	if err := stream.EnsureGroup(context.Background()); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	_, err := stream.Enqueue(context.Background(), map[string]any{
		constants.CleanupFieldResourceType: finalizers.ResourceTypeUsers,
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
	enqueueJob(t, stream, "u1")

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	m.Start(context.Background())
	defer m.Stop()

	if !waitForDLQ(rdb) {
		t.Fatal("job never reached the DLQ after exhausting its attempts")
	}
}

// A job whose consumer died mid-pass stays pending in the group and is invisible
// to a plain read; the worker reclaims it once it has idled past the threshold
// instead of waiting for the dedup key to lapse and the sweeper to re-enqueue.
func TestManagerReclaimsStalePendingJob(t *testing.T) {
	rdb, mr := testutil.RedisClient(t)
	cfg := fastConfig()
	cfg.XClaimMinIdle = testCallTimeout
	resourceType := finalizers.ResourceTypeUsers
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), resourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	enqueueJob(t, stream, "u3")

	deadConsumer := workerNameFor("replica-dead")
	if msgs, err := stream.Read(context.Background(), deadConsumer); err != nil || len(msgs) != constants.DefaultIncrementValue {
		t.Fatalf("dead consumer read = (%d msgs, %v), want the job delivered and left pending", len(msgs), err)
	}
	time.Sleep(cfg.XClaimMinIdle + cfg.XClaimMinIdle)
	mr.FastForward(cfg.XClaimMinIdle + cfg.XClaimMinIdle)

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	m.Start(context.Background())
	defer m.Stop()

	if !waitForDLQ(rdb) {
		t.Fatal("pending job was never reclaimed (it never reached the DLQ)")
	}
}

// Requeues wait initial, 2x initial, ... (capped) before the next attempt, so a
// job exhausting three attempts cannot reach the DLQ before the sum of its waits.
func TestManagerBacksOffBetweenAttempts(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	cfg := fastConfig()
	cfg.BackoffInitial = testBackoffInitial
	cfg.BackoffMax = time.Second
	resourceType := finalizers.ResourceTypeUsers
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), resourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)
	enqueueJob(t, stream, "u4")

	start := time.Now()
	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	m.Start(context.Background())
	defer m.Stop()

	if !waitForDLQ(rdb) {
		t.Fatal("job never reached the DLQ")
	}
	minWait := testBackoffInitial + testBackoffInitial + testBackoffInitial // 1x then 2x before the last attempt
	if elapsed := time.Since(start); elapsed < minWait {
		t.Fatalf("job reached the DLQ after %v, want at least %v of backoff", elapsed, minWait)
	}
}

// While Redis keeps failing, a worker waits initial, 2x, 4x... between passes instead of
// retrying every read block, so an outage logs a few lines a minute, not several a second.
func TestManagerBacksOffWhileRedisFails(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	reads := make(chan time.Time, failedReadsToWatch)
	rdb.AddHook(redisDown{reads: reads})
	cfg := fastConfig()
	cfg.BackoffInitial = testBackoffInitial
	cfg.BackoffMax = time.Minute
	resourceType := finalizers.ResourceTypeUsers
	stream := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), resourceType, cfg.StreamMaxLen, cfg.XClaimMinIdle)

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	m.Start(context.Background())
	defer m.Stop()

	var at []time.Time
	deadline := time.After(3 * time.Second)
	for len(at) < failedReadsToWatch {
		select {
		case read := <-reads:
			at = append(at, read)
		case <-deadline:
			t.Fatalf("%d failed reads in 3s, want %d spaced by a growing backoff", len(at), failedReadsToWatch)
		}
	}
	for i := constants.DefaultIncrementValue; i < len(at); i++ {
		want := testBackoffInitial << (i - constants.DefaultIncrementValue)
		if gap := at[i].Sub(at[i-constants.DefaultIncrementValue]); gap < want {
			t.Fatalf("failed read %d came %v after the previous one, want at least %v", i, gap, want)
		}
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
	enqueueJob(t, stream, "u2")

	m := cleanup.NewManager(cfg, resourceType, stream, cleanup.NewDedup(rdb, cfg.DedupTTL), newReconciler(cfg), testReplicaID)
	loop := cleanup.NewLeaderLoop(nil, []*cleanup.Manager{m}, nil, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		loop.Run(ctx)
		close(done)
	}()

	drained := waitForDLQ(rdb)
	cancel()
	<-done

	if !drained {
		t.Fatal("leader loop did not start the manager (job never reached the DLQ)")
	}
}
