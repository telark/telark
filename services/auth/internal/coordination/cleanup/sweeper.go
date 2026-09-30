package cleanup

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
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
		if v == nil {
			continue
		}
		if v.IsDeleting() {
			if v.HasFinalizer(ops.Finalizer) {
				s.enqueue(parent, v.Name)
			}
			continue
		}
		// The uninstall hook strips finalizers so a full teardown never hangs; a reinstall
		// that kept the data gets them back here.
		if !v.HasFinalizer(ops.Finalizer) {
			s.restoreFinalizer(parent, ops, v.Name)
		}
	}
}

func (s *Sweeper) restoreFinalizer(parent context.Context, ops cleanupctrl.ResourceOps, id string) {
	ctx, cancel := context.WithTimeout(parent, s.cfg.PatchTimeout)
	defer cancel()
	status := constants.DefaultInitValue
	if resp := ops.AddFinalizer(ctx, id, ops.Finalizer); resp != nil {
		status = resp.Status
	}
	switch status {
	case http.StatusOK:
		lg.Info(fmt.Sprintf(string(constants.LogCleanupFinalizerRestored), s.resourceType, id))
	case http.StatusNotFound:
	default:
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupAddFinalizerFailed), s.resourceType, id, status))
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
		lg.Debug(fmt.Sprintf(string(constants.LogCleanupSweeperEnqueued), s.resourceType, id))
	}
}
