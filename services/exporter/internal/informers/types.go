package informers

import k8scache "k8s.io/client-go/tools/cache"

type Source struct {
	Store  k8scache.Store
	Synced k8scache.InformerSynced
}
