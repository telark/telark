package informers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/constants"
	kcoredynamic "github.com/telark/kcore/informers/dynamic"
	"github.com/telark/kcore/k8sclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	k8scache "k8s.io/client-go/tools/cache"
)

var (
	lg           = constants.GetLogger(constants.PrefixInformers)
	applications atomic.Pointer[Source]
)

// Use installs the store list renders read from; a test hands in a filled one.
func Use(store k8scache.Store, synced k8scache.InformerSynced) {
	applications.Store(&Source{Store: store, Synced: synced})
}

func ApplicationsSynced() bool {
	src := applications.Load()
	return src != nil && src.Store != nil && src.Synced != nil && src.Synced()
}

// The items share their maps with the store, which client-go replaces and never
// mutates, so a caller that prunes must copy first. Sorted by name like a LIST.
func ListApplications() (*unstructured.UnstructuredList, bool) {
	if !ApplicationsSynced() {
		return nil, false
	}
	objects := applications.Load().Store.List()
	list := &unstructured.UnstructuredList{Items: make([]unstructured.Unstructured, constants.DefaultInitValue, len(objects))}
	for _, obj := range objects {
		if app, ok := obj.(*unstructured.Unstructured); ok {
			list.Items = append(list.Items, *app)
		}
	}
	slices.SortFunc(list.Items, func(a, b unstructured.Unstructured) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	return list, true
}

func StartApplications(ctx context.Context) {
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrApplicationInformerStartFailed), err))
		return
	}
	RunApplications(ctx, dyn)
}

// RunApplications watches the exporter's namespace, the one ListCustomResources
// reads, until ctx is canceled. It returns once the store has synced or the sync
// timeout passed; until then lists fall back to a LIST.
func RunApplications(ctx context.Context, dyn dynamic.Interface) {
	md := metadata.ApplicationAsResourceMetadata
	factory := kcoredynamic.NewNamespacedInformerFactory(dyn, constants.InformerNoResync, md.Namespace)
	informer := factory.ForResource(schema.GroupVersionResource{
		Group:    md.BaseGroup,
		Version:  md.Version,
		Resource: md.Plural,
	}).Informer()
	// managedFields often outweigh the spec and are never served.
	_ = informer.SetTransform(func(obj any) (any, error) {
		if app, ok := obj.(*unstructured.Unstructured); ok {
			app.SetManagedFields(nil)
		}
		return obj, nil
	})
	Use(informer.GetStore(), informer.HasSynced)
	factory.Start(ctx.Done())
	start := time.Now()
	syncCtx, cancel := context.WithTimeout(ctx, constants.InformerSyncTimeout)
	defer cancel()
	if !k8scache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced) {
		lg.Warn(fmt.Sprintf(string(constants.WarnApplicationInformerNotSynced), constants.InformerSyncTimeout))
		return
	}
	lg.Info(fmt.Sprintf(
		string(constants.InfApplicationInformerSynced),
		len(informer.GetStore().List()),
		time.Since(start).Round(time.Millisecond),
	))
}
