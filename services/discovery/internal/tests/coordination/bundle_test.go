package coordination

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/coordination"
)

// The bundle wires one of every Redis coordination client off a single
// connection.
func TestNewCoordinationBundle(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	bundle := coordination.NewCoordinationBundle(rdb, "replica-1", config.LoadCoordinationConfig())
	if bundle == nil {
		t.Fatal("nil bundle")
	}
	if bundle.Stream == nil || bundle.Lock == nil || bundle.State == nil ||
		bundle.Dedup == nil || bundle.Grace == nil || bundle.Election == nil {
		t.Fatalf("bundle has a nil client: %+v", bundle)
	}
}
