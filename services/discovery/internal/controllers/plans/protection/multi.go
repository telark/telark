package protection

import (
	"context"
	"sync"
	"time"

	"github.com/telark/discovery/internal/constants"
)

func StartLeaderGated(ctx context.Context, ctrl *Controller, isLeader func(context.Context) bool) {
	go runLeaderGated(ctx, ctrl, isLeader)
}

func runLeaderGated(ctx context.Context, ctrl *Controller, isLeader func(context.Context) bool) {
	t := time.NewTicker(constants.DiscoveryLeaderGatePoll)
	defer t.Stop()
	var runCancel context.CancelFunc
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			if runCancel != nil {
				runCancel()
				wg.Wait()
			}
			return
		case <-t.C:
			now := isLeader(ctx)
			if now && runCancel == nil {
				runCtx, cancel := context.WithCancel(ctx)
				runCancel = cancel
				wg.Go(func() {
					ctrl.Run(runCtx)
				})
			} else if !now && runCancel != nil {
				runCancel()
				wg.Wait()
				runCancel = nil
			}
		}
	}
}
