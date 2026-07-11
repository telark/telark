package forcesync

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
)

func NewMaintenance(cfg config.ForceSyncConfig, stream *StreamOps, replicaID string) *Maintenance {
	return &Maintenance{cfg: cfg, stream: stream, replicaID: replicaID}
}

func (m *Maintenance) Run(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.MaintenanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *Maintenance) tick(ctx context.Context) {
	reclaimed, err := m.stream.Reclaim(ctx, m.replicaID)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logDedup.ErrorOnce(constants.ForceSyncLogScopeMaintReclaim, string(constants.ErrForceSyncAutoClaimFailed), err)
	}
	if count := len(reclaimed); count > constants.DefaultInitValue {
		lg.Info(fmt.Sprintf(string(constants.LogForceSyncMaintenanceClaimed), count))
	}
	cutoff := time.Now().UTC()
	if err := m.stream.TrimByAge(ctx, cutoff); err != nil {
		if ctx.Err() != nil {
			return
		}
		logDedup.ErrorOnce(constants.ForceSyncLogScopeMaintTrim, string(constants.ErrForceSyncTrimFailed), err)
		return
	}
	trimmedBefore := cutoff.Add(-m.cfg.AckRetention).Format(time.RFC3339)
	lg.Info(fmt.Sprintf(string(constants.LogForceSyncMaintenanceTrimmed), trimmedBefore))
}
