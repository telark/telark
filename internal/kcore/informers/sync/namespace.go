package sync

import (
	"context"
	"time"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var logger = k8sclient.GetLogger()

func WaitForNamespaceCacheSync(ctx context.Context, informerFactory informers.SharedInformerFactory, timeout time.Duration) {
	informerFactory.Start(ctx.Done())
	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !cache.WaitForCacheSync(syncCtx.Done(),
		informerFactory.Core().V1().Namespaces().Informer().HasSynced,
	) {
		logger.Error(string(constants.ERROR_NAMESPACE_INFORMER_FAILED_TO_SYNC))
	} else {
		logger.Info(string(constants.INFO_NAMESPACE_INFORMER_SYNCED))
	}
}
