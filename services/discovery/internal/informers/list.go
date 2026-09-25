package informers

import (
	"context"
	"maps"
	"slices"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	kcoregroup "github.com/telark/kcore/resources/group"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
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

// Grouping names an app after its objects' identity labels, across namespaces.
// ponytail: one full cache pass per call; index objects by app key if that shows at thousands of apps.
func (m *Manager) namespacesOfApp(ctx context.Context, appName string) []string {
	if m == nil || appName == constants.EmptyString {
		return nil
	}
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
	found := make(map[string]struct{})
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	for _, inf := range m.informers {
		for _, it := range inf.GetIndexer().List() {
			u, ok := it.(*unstructured.Unstructured)
			if ok && u != nil && derivation.AppKey(u.GetLabels()) == appName && !slices.Contains(excluded, u.GetNamespace()) {
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
