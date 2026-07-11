package rollback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/kcore/k8sclient"
	"k8s.io/client-go/kubernetes"
)

var logger = constants.GetLogger(constants.LoggerPrefixRollbackController)

func StartLeaderGated(ctx context.Context, isLeader func(context.Context) bool) error {
	kubeClient, err := k8sclient.InitKubernetesClient()
	if err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrRollbackKubeClientInitFailed), err))
		return err
	}
	if kubeClient == nil {
		return errors.New(string(constants.ErrRollbackKubeClientNil))
	}
	go runLeaderGated(ctx, kubeClient, isLeader)
	return nil
}

func startController(ctx context.Context, kubeClient *kubernetes.Clientset) {
	if kubeClient == nil {
		logger.Error(string(constants.ErrRollbackKubeClientNilStart))
		return
	}
	ctrl := NewController(kubeClient)
	ctrl.Run(ctx)
}

func runLeaderGated(ctx context.Context, kubeClient *kubernetes.Clientset, isLeader func(context.Context) bool) {
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
					startController(runCtx, kubeClient)
				})
			} else if !now && runCancel != nil {
				runCancel()
				wg.Wait()
				runCancel = nil
			}
		}
	}
}
