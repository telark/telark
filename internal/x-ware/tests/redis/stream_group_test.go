package redis

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/x-ware/redis/stream"
)

const (
	groupStream   = "events"
	groupFromNow  = "from-now"
	groupFromZero = "from-start"
	groupReader   = "reader"
	readCount     = 10
	noBlock       = -1
	entryField    = "n"
)

// A group created from now skips the backlog; one created from the start reads it; ensuring
// an existing group again keeps its position.
func TestEnsureConsumerGroupFromNow(t *testing.T) {
	ctx := context.Background()
	rdb := goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	client := stream.NewStreamClient(rdb)

	publish := func(value string) {
		t.Helper()
		if _, err := client.Publish(ctx, groupStream, map[string]any{entryField: value}); err != nil {
			t.Fatal(err)
		}
	}
	publish("backlog")
	if err := client.EnsureConsumerGroupFromNow(ctx, groupStream, groupFromNow); err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureConsumerGroup(ctx, groupStream, groupFromZero); err != nil {
		t.Fatal(err)
	}
	publish("new")
	if err := client.EnsureConsumerGroupFromNow(ctx, groupStream, groupFromNow); err != nil {
		t.Fatalf("ensuring an existing group: %v", err)
	}

	fromNow, err := client.Consume(ctx, groupStream, groupFromNow, groupReader, readCount, noBlock)
	if err != nil || len(fromNow) != one || fromNow[none].Values[entryField] != "new" {
		t.Errorf("from-now group read %v (err %v), want only the new entry", fromNow, err)
	}
	fromZero, err := client.Consume(ctx, groupStream, groupFromZero, groupReader, readCount, noBlock)
	if err != nil || len(fromZero) != one+one {
		t.Errorf("from-start group read %d entries (err %v), want 2", len(fromZero), err)
	}
}
