package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/telark/telark/internal/data/logger"
	resourcesshared "github.com/telark/telark/internal/data/resources/shared"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
)

type stats struct {
	scanned int
	patched int
	skipped int
}

func RunBackFill(cfg config.BackfillConfig, lg *logger.CustomLogger) error {
	lg.Info(fmt.Sprintf(
		string(constants.LogBackfillFinalizersStarted),
		cfg.BatchSize, cfg.BatchPause.Milliseconds(),
	))
	totals := stats{}
	for _, resourceType := range cleanupctrl.RegisteredResourceTypes() {
		s, err := runForType(resourceType, cfg, lg)
		if err != nil {
			return err
		}
		totals.scanned += s.scanned
		totals.patched += s.patched
		totals.skipped += s.skipped
	}
	lg.Info(fmt.Sprintf(
		string(constants.LogBackfillFinalizersDone),
		totals.scanned, totals.patched, totals.skipped,
	))
	return nil
}

func runForType(resourceType string, cfg config.BackfillConfig, lg *logger.CustomLogger) (stats, error) {
	ops, ok := cleanupctrl.GetResourceOps(resourceType)
	if !ok {
		return stats{}, fmt.Errorf(string(constants.ErrBackfillUnknownResourceType), resourceType)
	}
	views, err := ops.List(context.Background())
	if err != nil {
		return stats{}, fmt.Errorf(string(constants.ErrBackfillListFailed), resourceType, err)
	}
	return patchBatches(resourceType, ops, views, cfg, lg), nil
}

func patchBatches(
	resourceType string,
	ops cleanupctrl.ResourceOps,
	views []*resourcesshared.CleanupView,
	cfg config.BackfillConfig,
	lg *logger.CustomLogger,
) stats {
	s := stats{}
	count := constants.DefaultInitValue
	for _, v := range views {
		if v == nil {
			continue
		}
		s.scanned++
		if v.HasFinalizer(ops.Finalizer) {
			s.skipped++
			continue
		}
		applyOne(resourceType, ops, v.Name, lg, &s)
		count++
		if cfg.BatchSize > constants.DefaultInitValue && count%cfg.BatchSize == constants.DefaultInitValue {
			time.Sleep(cfg.BatchPause)
		}
	}
	return s
}

func applyOne(
	resourceType string, ops cleanupctrl.ResourceOps, id string, lg *logger.CustomLogger, s *stats,
) {
	resp := ops.AddFinalizer(context.Background(), id, ops.Finalizer)
	if resp == nil || (resp.Status != http.StatusOK && resp.Status != http.StatusNotFound) {
		status := constants.DefaultInitValue
		if resp != nil {
			status = resp.Status
		}
		lg.Error(fmt.Sprintf(
			string(constants.ErrBackfillAddFinalizerFailed),
			resourceType, id, status,
		))
		return
	}
	s.patched++
}
