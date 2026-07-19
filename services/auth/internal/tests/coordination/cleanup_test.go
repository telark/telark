package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/telark/auth/internal/coordination/cleanup"
	"github.com/telark/auth/internal/tests/testutil"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

// Dedup claims a resource for one job at a time: the first claim wins, a second
// job sees the incumbent owner, and Release frees it for the next claimant.
func TestDedup(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	d := cleanup.NewDedup(rdb, time.Minute)
	ctx := context.Background()

	ok, owner, err := d.TryClaim(ctx, "user", "u1", "job1")
	if err != nil || !ok || owner != "job1" {
		t.Fatalf("first claim = (%v,%q,%v), want (true,job1,nil)", ok, owner, err)
	}

	ok, owner, err = d.TryClaim(ctx, "user", "u1", "job2")
	if err != nil || ok || owner != "job1" {
		t.Fatalf("contended claim = (%v,%q,%v), want (false,job1,nil)", ok, owner, err)
	}

	if err := d.Release(ctx, "user", "u1"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	ok, _, _ = d.TryClaim(ctx, "user", "u1", "job3")
	if !ok {
		t.Fatal("claim after release should succeed")
	}
}

// StreamOps drives the per-resource cleanup stream: ensure the group, enqueue a
// job, consume + ack it, and route to the DLQ. Missing-group recovery is
// exercised by reading before the group exists.
func TestStreamOps(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	ops := cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), "user", 100, time.Minute)
	ctx := context.Background()

	// Read before EnsureGroup exercises the NOGROUP recovery path.
	if _, err := ops.Read(ctx, "consumer-1"); err != nil {
		t.Fatalf("Read (with recovery) = %v", err)
	}

	id, err := ops.Enqueue(ctx, map[string]any{"resourceID": "u1"})
	if err != nil || id == "" {
		t.Fatalf("Enqueue = (%q,%v)", id, err)
	}

	msgs, err := ops.Read(ctx, "consumer-1")
	if err != nil || len(msgs) == 0 {
		t.Fatalf("Read = (%d msgs, %v)", len(msgs), err)
	}
	if err := ops.Ack(ctx, msgs[0].ID); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	if err := ops.PublishDLQ(ctx, map[string]any{"reason": "exhausted"}); err != nil {
		t.Fatalf("PublishDLQ: %v", err)
	}
	if _, err := ops.Reclaim(ctx, "consumer-1"); err != nil {
		t.Fatalf("Reclaim: %v", err)
	}
}

// Ingress is the enqueue front door: a new resource is claimed + streamed, a
// second request for the same resource is de-duplicated to the incumbent job,
// and an unknown resource type is rejected.
func TestIngressEnqueue(t *testing.T) {
	rdb, _ := testutil.RedisClient(t)
	streams := map[string]*cleanup.StreamOps{
		"user": cleanup.NewStreamOps(xwareredis.NewStreamClient(rdb), "user", 100, time.Minute),
	}
	ing := cleanup.NewIngress(streams, cleanup.NewDedup(rdb, time.Minute))
	ctx := context.Background()

	res, err := ing.Enqueue(ctx, cleanup.EnqueueRequest{ResourceType: "user", ResourceID: "u1", RequestedBy: "admin"})
	if err != nil || !res.Enqueued || res.JobID == "" {
		t.Fatalf("first enqueue = (%+v, %v), want enqueued", res, err)
	}

	dup, err := ing.Enqueue(ctx, cleanup.EnqueueRequest{ResourceType: "user", ResourceID: "u1"})
	if err != nil || dup.Enqueued {
		t.Fatalf("duplicate enqueue = (%+v, %v), want not enqueued", dup, err)
	}
	testutil.Equal(t, "dedup returns incumbent job", dup.JobID, res.JobID)

	if _, err := ing.Enqueue(ctx, cleanup.EnqueueRequest{ResourceType: "unknown", ResourceID: "x"}); err == nil {
		t.Fatal("enqueue for unknown resource type should error")
	}
}
