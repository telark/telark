package informers

import (
	"context"
	"slices"

	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	kcoregroup "github.com/telark/telark/internal/kcore/resources/group"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Resolves the group from the informer cache, never the API: the HasSynced guard in
// attachInformer makes the cache complete before any handler runs.
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
	return resourceRefKey(applicationmodel.Resource{Namespace: u.GetNamespace(), Kind: u.GetKind(), Name: u.GetName()})
}

func resourceRefKey(r applicationmodel.Resource) string {
	return r.Namespace + constants.PathSeparator + r.Kind + constants.PathSeparator + r.Name
}
