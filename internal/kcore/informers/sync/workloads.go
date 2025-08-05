package sync

import (
	"context"
	"time"

	resourceshared "github.com/plsyro/data/resources/shared"
	appworkload "github.com/plsyro/data/resources/workloads/app"
	"github.com/plsyro/kcore/constants"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func WaitForWorkloadsCacheSync(ctx context.Context, workloads []*appworkload.AppWorkloadAsResource, informerFactory informers.SharedInformerFactory, timeout time.Duration) {
	informerFactory.Start(ctx.Done())
	workloadTypes := make(map[string]bool)
	for _, workload := range workloads {
		workloadTypes[string(workload.Fasid.SourceType)] = true
	}

	var informersToSync []cache.InformerSynced

	if workloadTypes[string(resourceshared.Deploy)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().Deployments().Informer().HasSynced)
	}

	if workloadTypes[string(resourceshared.StatefulSet)] {
		informersToSync = append(informersToSync, informerFactory.Apps().V1().StatefulSets().Informer().HasSynced)
	}

	if workloadTypes[string(resourceshared.DaemonSet)] {
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
