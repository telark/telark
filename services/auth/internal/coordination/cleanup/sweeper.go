package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
)

func NewSweeper(cfg config.CleanupConfig, resourceType string, ingress *Ingress) *Sweeper {
	return &Sweeper{cfg: cfg, resourceType: resourceType, ingress: ingress}
}

func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.SweeperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Sweeper) tick(parent context.Context) {
	ops, ok := cleanupctrl.GetResourceOps(s.resourceType)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(parent, s.cfg.ListTimeout)
	defer cancel()
	views, err := ops.List(ctx)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupSweeperListFailed), s.resourceType, err))
		return
	}
	for _, v := range views {
		if v == nil || !v.IsDeleting() || !v.HasFinalizer(ops.Finalizer) {
			continue
		}
		s.enqueue(parent, v.Name)
	}
}

func (s *Sweeper) enqueue(parent context.Context, id string) {
	enqCtx, cancel := context.WithTimeout(parent, s.cfg.PatchTimeout)
	defer cancel()
	result, err := s.ingress.Enqueue(enqCtx, EnqueueRequest{
		ResourceType: s.resourceType,
		ResourceID:   id,
		RequestedBy:  constants.CleanupRequestedBySweeper,
	})
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupEnqueueFailed), s.resourceType, id, err))
		return
	}
	if result.Enqueued {
		lg.Info(fmt.Sprintf(string(constants.LogCleanupSweeperEnqueued), s.resourceType, id))
	}
}
