package coordination

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

func AdvertiseReplica(ctx context.Context, rdb *redis.Client, replicaID, addr string) {
	if rdb == nil {
		return
	}
	pipe := rdb.Pipeline()
	pipe.Set(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaHB,
		time.Now().UTC().String(), xwareredis.ReplicaHeartbeatTTL)
	// A replica ID is a pod name, which nothing resolves; peers dial this instead.
	if addr != constants.EmptyString {
		pipe.Set(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaAddr,
			addr, xwareredis.ReplicaHeartbeatTTL)
	}
	_, _ = pipe.Exec(ctx)
}

func ReplicaAddress(ctx context.Context, rdb *redis.Client, replicaID string) string {
	if rdb == nil {
		return constants.EmptyString
	}
	addr, _ := rdb.Get(ctx, constants.KeyPrefixReplicaHB+replicaID+constants.KeySuffixReplicaAddr).Result()
	return addr
}
