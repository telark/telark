package coordination

import (
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
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
