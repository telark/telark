package sync

import (
	"context"
	"time"

	resourceShared "github.com/plsyro/data/resources/shared"
	appWorkload "github.com/plsyro/data/resources/workloads/app"
	"github.com/plsyro/kcore/constants"
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

	if workloadTypes[string(resourceShared.Deploy)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().Deployments().Informer().HasSynced)
	}

	if workloadTypes[string(resourceShared.StatefulSet)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().StatefulSets().Informer().HasSynced)
	}

	if workloadTypes[string(resourceShared.DaemonSet)] {
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
