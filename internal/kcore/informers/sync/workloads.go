package sync

import (
	"context"
	"time"

	resourcesCommon "github.com/plsyro/data-pkg/resources/common"
	appWorkload "github.com/plsyro/data-pkg/resources/workload/app"
	"github.com/plsyro/kcore-pkg/constants"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func WaitForWorkloadsCacheSync(ctx context.Context, workloads []*appWorkload.AppWorkloadAsResource, informerFactory informers.SharedInformerFactory, timeout time.Duration) {
	informerFactory.Start(ctx.Done())
	workloadTypes := make(map[string]bool)
	for _, workload := range workloads {
		workloadTypes[string(workload.Fasid.SourceType)] = true
	}

	var informersToSync []cache.InformerSynced

	if workloadTypes[string(resourcesCommon.DEPLOY)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().Deployments().Informer().HasSynced)
	}

	if workloadTypes[string(resourcesCommon.STATEFUL_SET)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().StatefulSets().Informer().HasSynced)
	}

	if workloadTypes[string(resourcesCommon.DAEMON_SET)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().DaemonSets().Informer().HasSynced)
	}

	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), informersToSync...) {
		logger.Error(string(constants.ErrWorkloadInformerFailedToSync))
	} else {
		logger.Info(string(constants.InfoWorkloadInformerSynced))
	}
}
