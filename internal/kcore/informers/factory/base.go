package factory

import (
	"fmt"
	"time"

	"github.com/telark/kcore/constants"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func CreateInformerFactory(
	kubeClient *kubernetes.Clientset,
	resyncPeriod time.Duration,
) (informers.SharedInformerFactory, error) {
	if kubeClient == nil {
		return nil, fmt.Errorf("%s", constants.ErrKubeClientNil)
	}
	return informers.NewSharedInformerFactoryWithOptions(
		kubeClient,
		resyncPeriod,
	), nil
}

func CreateServiceInformer(factory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	if factory == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrInformerFactoryNil)
	}
	return factory.Core().V1().Services().Informer(), nil
}

func CreateNamespaceInformer(factory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	if factory == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrInformerFactoryNil)
	}
	return factory.Core().V1().Namespaces().Informer(), nil
}

func CreateDeploymentInformer(factory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	if factory == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrInformerFactoryNil)
	}
	return factory.Apps().V1().Deployments().Informer(), nil
}

func CreateStatefulSetInformer(factory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	if factory == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrInformerFactoryNil)
	}
	return factory.Apps().V1().StatefulSets().Informer(), nil
}

func CreateDaemonSetInformer(factory informers.SharedInformerFactory) (cache.SharedIndexInformer, error) {
	if factory == nil {
		return nil, fmt.Errorf(constants.ErrorFormatString, constants.ErrInformerFactoryNil)
	}
	return factory.Apps().V1().DaemonSets().Informer(), nil
}
