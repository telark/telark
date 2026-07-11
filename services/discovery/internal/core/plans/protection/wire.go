package protection

import (
	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	kcorek8s "github.com/telark/kcore/k8sclient"
	"k8s.io/client-go/kubernetes"
)

func BuildService(kubeClient *kubernetes.Clientset, rdb *redis.Client, logger Logger) (*Service, error) {
	dyn, err := kcorek8s.InitDynamicClient()
	if err != nil {
		return nil, err
	}
	mapper := kcorek8s.NewDeferredRESTMapper(kubeClient)
	applier := protpolicies.NewApplier(dyn, mapper)
	resolver := applications.NewRedisResolver(rdb)
	exporter := clients.NewProtectionPlanClient()
	return NewService(applier, resolver, exporter, dyn, logger), nil
}
