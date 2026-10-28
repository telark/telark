package coordination

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
)

const (
	syncReplica = "replica-1"
	blankApp    = " "
)

// The lock is released whatever the sync returns, so the next force-sync never waits out its TTL.
func TestRunForceSyncJobReleasesItsLock(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()
	bundle := coordination.NewCoordinationBundle(rdb, syncReplica, config.LoadCoordinationConfig())

	if err := coordination.RunForceSyncJob(context.Background(), bundle, rdb, syncReplica, blankApp); err == nil {
		t.Fatal("a blank application name synced")
	}
	if mr.Exists(constants.KeyPrefixLockApp + blankApp) {
		t.Fatal("the force-sync lock outlived the job")
	}
}
