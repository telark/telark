package sync

import (
	"context"
	"time"

	"github.com/plsyro/kcore/constants"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

func WaitForServiceCacheSync(
	ctx context.Context,
	informerFactory informers.SharedInformerFactory,
	timeout time.Duration,
) {
	informerFactory.Start(ctx.Done())
	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if !cache.WaitForCacheSync(syncCtx.Done(),
		informerFactory.Core().V1().Services().Informer().HasSynced,
	) {
		logger.Error(string(constants.ErrServiceInformerFailedToSync))
	} else {
		logger.Info(string(constants.InfoServiceInformerSynced))
	}
}
