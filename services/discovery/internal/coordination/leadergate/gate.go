package leadergate

import (
	"context"
	"sync"
	"time"

	"github.com/telark/discovery/internal/constants"
)

type Runnable interface {
	Run(ctx context.Context)
}

// r runs for exactly as long as isLeader stays true and is canceled the moment leadership
// is lost, so a loop driven by this never runs on two replicas at once.
func Start(ctx context.Context, r Runnable, isLeader func(context.Context) bool) {
	go run(ctx, r, isLeader)
}

func run(ctx context.Context, r Runnable, isLeader func(context.Context) bool) {
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
			leader := isLeader(ctx)
			if leader && runCancel == nil {
				runCtx, cancel := context.WithCancel(ctx)
				runCancel = cancel
				wg.Go(func() {
					r.Run(runCtx)
				})
			} else if !leader && runCancel != nil {
				runCancel()
				wg.Wait()
				runCancel = nil
			}
		}
	}
}
