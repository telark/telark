package rollback

import (
	"context"
	"errors"
	"fmt"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/coordination/leadergate"
	"github.com/telark/kcore/k8sclient"
)

var logger = constants.GetLogger(constants.LoggerPrefixRollbackController)

// StartLeaderGated runs the rollback controller on the leader. Client init lives
// here because it can fail; the leader gating itself is the shared one.
func StartLeaderGated(ctx context.Context, isLeader func(context.Context) bool) error {
	kubeClient, err := k8sclient.InitKubernetesClient()
	if err != nil {
		logger.Error(fmt.Sprintf(string(constants.ErrRollbackKubeClientInitFailed), err))
		return err
	}
	if kubeClient == nil {
		return errors.New(string(constants.ErrRollbackKubeClientNil))
	}
	leadergate.Start(ctx, NewController(kubeClient), isLeader)
	return nil
}
