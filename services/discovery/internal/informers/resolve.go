package informers

import (
	"context"

	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/discovery/derivation"
	discoveryshared "github.com/telark/discovery/discovery/shared"
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
	if !ok || len(refs) == constants.DefaultInitValue {
		return constants.EmptyString
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
