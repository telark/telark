package forcesync

import (
	"context"
	"fmt"
	"time"

	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func NewLeaderLoop(
	election *xwareredis.ElectionClient,
	manager *Manager,
	maintenance *Maintenance,
	cfg config.CoordinationConfig,
) *LeaderLoop {
	return &LeaderLoop{
		election:    election,
		manager:     manager,
		maintenance: maintenance,
		cfg:         cfg,
	}
}

func (l *LeaderLoop) Run(parent context.Context) {
	ticker := time.NewTicker(l.cfg.ElectionRenewInterval)
	defer ticker.Stop()
	defer l.stop()

	for {
		select {
		case <-parent.Done():
			return
		case <-ticker.C:
			l.reconcileLeadership(parent)
		}
	}
}

func (l *LeaderLoop) reconcileLeadership(parent context.Context) {
	leader := l.checkLeader(parent)
	if leader && !l.active {
		l.start(parent)
		return
	}
	if !leader && l.active {
		l.stop()
	}
}

func (l *LeaderLoop) start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	l.cancel = cancel
	l.active = true
	l.manager.Start(ctx)
	go l.maintenance.Run(ctx)
}

func (l *LeaderLoop) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	if l.active {
		l.manager.Stop()
		l.active = false
	}
}

func (l *LeaderLoop) checkLeader(ctx context.Context) bool {
	ok, err := l.election.IsLeader(ctx)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrForceSyncReadGroupFailed), err))
		return false
	}
	return ok
}
