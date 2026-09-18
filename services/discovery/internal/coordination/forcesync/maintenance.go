package forcesync

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
)

func NewMaintenance(cfg config.ForceSyncConfig, stream *StreamOps) *Maintenance {
	return &Maintenance{cfg: cfg, stream: stream}
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
