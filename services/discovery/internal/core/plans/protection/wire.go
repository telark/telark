package protection

import (
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/core/plans/protection/applications"
	protpolicies "github.com/telark/discovery/core/plans/protection/policies"
	kcorek8s "github.com/telark/kcore/k8sclient"
	"github.com/redis/go-redis/v9"
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
