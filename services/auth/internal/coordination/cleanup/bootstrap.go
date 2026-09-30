package cleanup

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/resources/finalizers"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
)

func Bootstrap(
	ctx context.Context,
	rdb *redis.Client,
	cfg config.CleanupConfig,
	reconciler *cleanupctrl.Reconciler,
	election *xwareredis.ElectionClient,
	replicaID string,
	leaderRenewTick time.Duration,
) (*System, error) {
	streamClient := xwareredis.NewStreamClient(rdb)
	dedup := NewDedup(rdb, cfg.DedupTTL)

	resourceTypes := []string{
		finalizers.ResourceTypeUsers,
		finalizers.ResourceTypeGroups,
		finalizers.ResourceTypeRoles,
	}

	streams := make(map[string]*StreamOps, len(resourceTypes))
	managers := make([]*Manager, constants.DefaultInitValue, len(resourceTypes))
	sweepers := make([]*Sweeper, constants.DefaultInitValue, len(resourceTypes))

	ingress := NewIngress(streams, dedup)

	for _, rt := range resourceTypes {
		ops := NewStreamOps(streamClient, rt, cfg.StreamMaxLen, cfg.XClaimMinIdle)
		if err := ops.EnsureGroup(ctx); err != nil {
			return nil, err
		}
		streams[rt] = ops
		managers = append(managers, NewManager(cfg, rt, ops, dedup, reconciler, replicaID))
		sweepers = append(sweepers, NewSweeper(cfg, rt, ingress))
	}

	loop := NewLeaderLoop(election, managers, sweepers, leaderRenewTick)
	return &System{Ingress: ingress, LeaderLoop: loop}, nil
}
