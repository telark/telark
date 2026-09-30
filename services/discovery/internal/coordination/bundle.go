package coordination

import (
	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
)

type CoordinationBundle struct {
	Stream   *xwareredis.StreamClient
	Lock     *xwareredis.LockClient
	State    *xwareredis.StateClient
	Dedup    *xwareredis.DedupClient
	Grace    *xwareredis.GraceClient
	Election *xwareredis.ElectionClient
	Config   config.CoordinationConfig
}

func NewCoordinationBundle(r *redis.Client, replicaID string, cfg config.CoordinationConfig) *CoordinationBundle {
	return &CoordinationBundle{
		Stream: xwareredis.NewStreamClient(r),
		Lock:   xwareredis.NewLockClient(r),
		State:  xwareredis.NewStateClient(r),
		Dedup:  xwareredis.NewDedupClient(r),
		Grace:  xwareredis.NewGraceClient(r),
		Election: xwareredis.NewElectionClient(
			r,
			replicaID,
			constants.KeyElectionPrewarm,
			cfg.ElectionTTL,
		),
		Config: cfg,
	}
}
