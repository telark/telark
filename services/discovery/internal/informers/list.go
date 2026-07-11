package informers

import (
	"context"
	"slices"

	"github.com/telark/discovery/internal/constants"
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

func refFromUnstructured(u *unstructured.Unstructured) kcoregroup.ResourceRef {
	return kcoregroup.ResourceRef{
		Namespace: u.GetNamespace(),
		Kind:      u.GetKind(),
		Name:      u.GetName(),
		Labels:    u.GetLabels(),
	}
}

func (m *Manager) getCachedManifest(kind, name, ns string) (map[string]any, bool) {
	if m == nil {
		return nil, false
	}
	key := ns + "/" + kind + "/" + name
	m.informersMu.RLock()
	defer m.informersMu.RUnlock()
	for _, inf := range m.informers {
		obj, exists, err := inf.GetIndexer().GetByKey(key)
		if err != nil || !exists || obj == nil {
			continue
		}
		u, ok := obj.(*unstructured.Unstructured)
		if !ok || u == nil {
			continue
		}
		cp := u.DeepCopy()
		return cp.Object, true
	}
	return nil, false
}
