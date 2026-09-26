package informers

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/telark/discovery/internal/constants"
	tcfghelper "github.com/telark/discovery/internal/helpers/telarkconfig"
	kcoredynamic "github.com/telark/kcore/informers/dynamic"
	"github.com/telark/kcore/resources/core"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamiciface "k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

func nonExcludedNamespaceNames(ctx context.Context) (map[string]struct{}, error) {
	nsList, err := core.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	ex := tcfghelper.FetchExcludedNamespaces(ctx)
	out := make(map[string]struct{}, len(nsList))
	for i := range nsList {
		name := strings.TrimSpace(nsList[i].Name)
		if name == constants.EmptyString {
			continue
		}
		if slices.Contains(ex, name) {
			continue
		}
		out[name] = struct{}{}
	}
	return out, nil
}

func informerIndexKey(ns string, gvr schema.GroupVersionResource) string {
	return ns + constants.InformerNsGVRDelim + gvr.String()
}

func (m *Manager) reconcileNamespaceWatchers(ctx context.Context, dyn dynamiciface.Interface, resync time.Duration) {
	if len(m.watchGVRs) == constants.DefaultInitValue {
		return
	}
	want, err := nonExcludedNamespaceNames(ctx)
	if err != nil {
		return
	}
	m.nsRunMu.Lock()
	for ns, cancel := range m.nsCancels {
		if _, keep := want[ns]; !keep {
			cancel()
			delete(m.nsCancels, ns)
		}
	}
	for ns := range want {
		if _, exists := m.nsCancels[ns]; exists {
			continue
		}

		nctx, cancel := context.WithCancel(ctx)
		m.nsCancels[ns] = cancel
		go m.runNamespaceInformerLoop(nctx, dyn, resync, ns)
	}
	m.nsRunMu.Unlock()
}

func (m *Manager) runNamespaceInformerLoop(
	ctx context.Context,
	dyn dynamiciface.Interface,
	resync time.Duration,
	ns string,
) {
	factory := kcoredynamic.NewNamespacedInformerFactory(dyn, resync, ns)
	syncers := make([]cache.InformerSynced, constants.DefaultInitValue, len(m.watchGVRs))
	for i := range m.watchGVRs {
		inf := m.attachInformer(factory, ns, m.watchGVRs[i])
		syncers = append(syncers, inf.HasSynced)
	}
	factory.Start(ctx.Done())
	start := time.Now()
	_ = cache.WaitForCacheSync(ctx.Done(), syncers...)
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Info(fmt.Sprintf(
		string(constants.InfoInformerNamespaceSynced), ns, len(syncers), time.Since(start).Round(time.Millisecond),
	))
	<-ctx.Done()
	m.removeInformersForNamespace(ns)
}

func (m *Manager) stopAllNamespaceWatchers() {
	m.nsRunMu.Lock()
	for _, cancel := range m.nsCancels {
		cancel()
	}
	m.nsCancels = make(map[string]context.CancelFunc)
	m.nsRunMu.Unlock()
}

func (m *Manager) removeInformersForNamespace(ns string) {
	prefix := ns + constants.InformerNsGVRDelim
	m.informersMu.Lock()
	defer m.informersMu.Unlock()
	for k := range m.informers {
		if strings.HasPrefix(k, prefix) {
			delete(m.informers, k)
		}
	}
}
