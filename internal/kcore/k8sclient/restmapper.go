package k8sclient

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

func NewDeferredRESTMapper(kubeClient *kubernetes.Clientset) meta.RESTMapper {
	return restmapper.NewDeferredDiscoveryRESTMapper(
		memory.NewMemCacheClient(kubeClient.Discovery()),
	)
}
