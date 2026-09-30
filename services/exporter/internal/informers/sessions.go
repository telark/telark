package informers

import (
	"context"
	"fmt"
	"sync/atomic"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	k8scache "k8s.io/client-go/tools/cache"
)

var sessions atomic.Pointer[Source]

func UseSessions(store k8scache.Store, synced k8scache.InformerSynced) {
	sessions.Store(&Source{Store: store, Synced: synced})
}

func SessionsSynced() bool {
	src := sessions.Load()
	return src != nil && src.Store != nil && src.Synced != nil && src.Synced()
}

// GetSession answers from the watch-fed mirror of the session records, so
// resolving an identity costs no apiserver call and never waits behind the
// shared client's limiter. Before sync, or on a miss, the caller asks the
// apiserver.
func GetSession(name string) (*unstructured.Unstructured, bool) {
	if !SessionsSynced() {
		return nil, false
	}
	key := fmt.Sprintf(constants.InformerKeyFormat, authmetadata.SessionMetadata.Namespace, name)
	obj, exists, err := sessions.Load().Store.GetByKey(key)
	if err != nil || !exists {
		return nil, false
	}
	session, ok := obj.(*unstructured.Unstructured)
	return session, ok
}

func StartSessions(ctx context.Context) {
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSessionInformerStartFailed), err))
		return
	}
	RunSessions(ctx, dyn)
}

func RunSessions(ctx context.Context, dyn dynamic.Interface) {
	runMirror(ctx, dyn, mirror{
		md:        authmetadata.SessionMetadata,
		install:   UseSessions,
		synced:    constants.InfSessionInformerSynced,
		notSynced: constants.WarnSessionInformerNotSynced,
	})
}
