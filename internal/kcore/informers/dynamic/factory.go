package dynamicinformers

import (
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

func NewSharedInformerFactory(client dynamic.Interface, resync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(client, resync)
}

func NewNamespacedInformerFactory(client dynamic.Interface, resync time.Duration, namespace string) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewFilteredDynamicSharedInformerFactory(client, resync, namespace, nil)
}
