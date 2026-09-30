package cleanup

import (
	"context"
	"fmt"
	"time"

	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/auth/internal/constants"
)

func NewLeaderLoop(
	election *xwareredis.ElectionClient,
	managers []*Manager,
	sweepers []*Sweeper,
	renewTick time.Duration,
) *LeaderLoop {
	return &LeaderLoop{
		election:  election,
		managers:  managers,
		sweepers:  sweepers,
		renewTick: renewTick,
	}
}

func (l *LeaderLoop) Run(parent context.Context) {
	ticker := time.NewTicker(l.renewTick)
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
	for _, m := range l.managers {
		m.Start(ctx)
	}
	for _, s := range l.sweepers {
		go s.Run(ctx)
	}
}

func (l *LeaderLoop) stop() {
	if l.cancel != nil {
		l.cancel()
		l.cancel = nil
	}
	if l.active {
		for _, m := range l.managers {
			m.Stop()
		}
		l.active = false
	}
}

func (l *LeaderLoop) checkLeader(ctx context.Context) bool {
	if l.election == nil {
		return true
	}
	ok, err := l.election.IsLeader(ctx)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupElectionCheckFailed), err))
		return false
	}
	return ok
}
