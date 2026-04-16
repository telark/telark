package dynamicinformers

import (
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

func NewSharedInformerFactory(client dynamic.Interface, resync time.Duration) dynamicinformer.DynamicSharedInformerFactory {
	return dynamicinformer.NewDynamicSharedInformerFactory(client, resync)
}
