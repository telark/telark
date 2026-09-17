package informers

import (
	"context"
	"slices"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	kcoregroup "github.com/telark/kcore/resources/group"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// appGroupFromCache resolves the application group for a K8s resource using
// the local informer cache instead of live K8s API calls. The cache is
// guaranteed to be synced before any event handlers fire (HasSynced guard
// in attachInformer), so this is always safe to call from onAdd/onUpdate.
func (m *Manager) appGroupFromCache(
	ctx context.Context,
	u *unstructured.Unstructured,
) string {
	if u == nil {
		return constants.EmptyString
	}
	ns := u.GetNamespace()
	if ns == constants.EmptyString {
		return constants.EmptyString
	}
	refs, ok := m.listRefsInNamespaces(ctx, map[string]struct{}{ns: {}})
	if !ok {
		return constants.EmptyString
	}
	// A deleted object has already left the cache; group it with what remains.
	if !slices.ContainsFunc(refs, func(r kcoregroup.ResourceRef) bool {
		return r.Namespace == ns && r.Kind == u.GetKind() && r.Name == u.GetName()
	}) {
		refs = append(refs, refFromUnstructured(u))
	}
	inputs := discoveryshared.ToDerivationInputs(refs)
	withGroups := derivation.GroupByWorkloadAnchor(inputs)
	kind := u.GetKind()
	name := u.GetName()
	for i := range withGroups {
		wg := &withGroups[i]
		if wg.Namespace == ns && wg.Kind == kind && wg.Name == name {
			return wg.Group
		}
	}
	return constants.EmptyString
}

func resourceKey(u *unstructured.Unstructured) string {
	if u == nil {
		return constants.EmptyString
	}
	return u.GetNamespace() + "/" + u.GetKind() + "/" + u.GetName()
}
