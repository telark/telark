package informers

import (
	"context"
	"maps"
	"slices"

	kcoregroup "github.com/telark/telark/internal/kcore/resources/group"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func TryListResourcesInNamespaces(ctx context.Context, namespaces []string) ([]kcoregroup.ResourceRef, bool) {
	m := Global()
	if m == nil {
		return nil, false
	}
	nsSet := make(map[string]struct{})
	for _, ns := range namespaces {
		if ns != constants.EmptyString {
			nsSet[ns] = struct{}{}
		}
	}
	out, ok := m.listRefsInNamespaces(ctx, nsSet)
	return out, ok
}

func (m *Manager) listRefsInNamespaces(
	ctx context.Context,
	nsSet map[string]struct{},
) ([]kcoregroup.ResourceRef, bool) {
	if m == nil || len(nsSet) == constants.DefaultInitValue {
		return nil, false
	}
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	if len(m.informers) == constants.DefaultInitValue {
		return nil, false
	}
	var out []kcoregroup.ResourceRef
	for _, inf := range m.informers {
		objs := inf.GetIndexer().List()
		for _, it := range objs {
			u, ok := it.(*unstructured.Unstructured)
			if !ok || u == nil {
				continue
			}
			ns := u.GetNamespace()
			if _, keep := nsSet[ns]; !keep {
				continue
			}
			if slices.Contains(excluded, ns) {
				continue
			}
			out = append(out, refFromUnstructured(u))
		}
	}
	return out, true
}

func AppNamespaces(ctx context.Context, appName string) []string {
	return Global().namespacesOfApp(ctx, appName)
}

// Synced informers indexing no object of the app: its resources were relabeled or
// deleted while their namespace lives, and nothing republishes the CR with zero resources.
func AppVanished(ctx context.Context, appName string) bool {
	m := Global()
	return m != nil && m.informersSynced() && len(m.namespacesOfApp(ctx, appName)) == constants.DefaultInitValue
}

// Grouping names an app after its objects' identity labels, across namespaces;
// the store indexes them by that key, so a lookup touches only the app's objects.
func appIndexers() cache.Indexers {
	return cache.Indexers{constants.InformerAppIndex: func(obj any) ([]string, error) {
		u, ok := obj.(*unstructured.Unstructured)
		if !ok || u == nil {
			return nil, nil
		}
		if key := derivation.AppKey(u.GetLabels()); key != constants.EmptyString {
			return []string{key}, nil
		}
		return nil, nil
	}}
}

func (m *Manager) namespacesOfApp(ctx context.Context, appName string) []string {
	if m == nil || appName == constants.EmptyString {
		return nil
	}
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	found := make(map[string]struct{})
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	for _, inf := range m.informers {
		objs, _ := inf.GetIndexer().ByIndex(constants.InformerAppIndex, appName)
		for _, it := range objs {
			u, ok := it.(*unstructured.Unstructured)
			if ok && u != nil && !slices.Contains(excluded, u.GetNamespace()) {
				found[u.GetNamespace()] = struct{}{}
			}
		}
	}
	return slices.Sorted(maps.Keys(found))
}

func refFromUnstructured(u *unstructured.Unstructured) kcoregroup.ResourceRef {
	cms, secs := kcoregroup.WorkloadConfigRefs(u)
	return kcoregroup.ResourceRef{
		Namespace:     u.GetNamespace(),
		Kind:          u.GetKind(),
		Name:          u.GetName(),
		Labels:        u.GetLabels(),
		Owners:        kcoregroup.OwnersOf(u),
		ConfigMapRefs: cms,
		SecretRefs:    secs,
	}
}

func (m *Manager) getCachedManifest(kind, name, ns string) (map[string]any, bool) {
	u, ok := m.cachedObject(kind, name, ns)
	if !ok {
		return nil, false
	}
	return u.DeepCopy().Object, true
}

// cachedObject hands out the informer's own object: read it, never mutate it.
func (m *Manager) cachedObject(kind, name, ns string) (*unstructured.Unstructured, bool) {
	if m == nil {
		return nil, false
	}
	// Indexers key by namespace/name; the kind is checked on the hit because
	// every watched resource of the namespace shares the same key space.
	key := ns + "/" + name
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	for _, inf := range m.informers {
		obj, exists, err := inf.GetIndexer().GetByKey(key)
		if err != nil || !exists || obj == nil {
			continue
		}
		u, ok := obj.(*unstructured.Unstructured)
		if !ok || u == nil || u.GetKind() != kind {
			continue
		}
		return u, true
	}
	return nil, false
}
