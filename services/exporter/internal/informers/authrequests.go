package informers

import (
	"context"
	"fmt"
	"sync/atomic"

	metadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	k8scache "k8s.io/client-go/tools/cache"
)

var authRequests atomic.Pointer[Source]

func UseAuthRequests(store k8scache.Store, synced k8scache.InformerSynced) {
	authRequests.Store(&Source{Store: store, Synced: synced})
}

func AuthRequestsSynced() bool {
	src := authRequests.Load()
	return src != nil && src.Store != nil && src.Synced != nil && src.Synced()
}

// The items share their maps with the store, so a caller that changes one must copy first. Before sync the caller
// asks the apiserver.
func ListAuthRequests() ([]unstructured.Unstructured, bool) {
	if !AuthRequestsSynced() {
		return nil, false
	}
	objects := authRequests.Load().Store.List()
	items := make([]unstructured.Unstructured, constants.DefaultInitValue, len(objects))
	for _, obj := range objects {
		if request, ok := obj.(*unstructured.Unstructured); ok {
			items = append(items, *request)
		}
	}
	return items, true
}

func StartAuthRequests(ctx context.Context) {
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrAuthRequestInformerStartFailed), err))
		return
	}
	RunAuthRequests(ctx, dyn)
}

func RunAuthRequests(ctx context.Context, dyn dynamic.Interface) {
	runMirror(ctx, dyn, mirror{
		md:        metadata.AuthRequestMetadata,
		install:   UseAuthRequests,
		synced:    constants.InfAuthRequestInformerSynced,
		notSynced: constants.WarnAuthRequestInformerNotSynced,
	})
}
