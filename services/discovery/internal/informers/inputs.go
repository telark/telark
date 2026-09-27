package informers

import (
	"context"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
)

func inputsForApp(
	ctx context.Context,
	m *Manager,
	stored *application.Application,
	appName string,
) []derivation.ResourceInput {
	if stored == nil {
		return nil
	}
	nsSet := make(map[string]struct{})
	for i := range stored.Namespaces.Items {
		ns := stored.Namespaces.Items[i].Name
		if ns != constants.EmptyString {
			nsSet[ns] = struct{}{}
		}
	}
	// The stored list alone never grows: an event in a namespace the store lost would be dropped.
	for _, ns := range m.namespacesOfApp(ctx, appName) {
		nsSet[ns] = struct{}{}
	}
	if len(nsSet) == constants.DefaultInitValue {
		return nil
	}
	refs, ok := m.listRefsInNamespaces(ctx, nsSet)
	if !ok || len(refs) == constants.DefaultInitValue {
		return nil
	}
	inputs := discoveryshared.ToDerivationInputs(refs)
	withGroups := derivation.GroupByWorkloadAnchor(inputs)
	out := make([]derivation.ResourceInput, constants.DefaultInitValue, len(withGroups))
	for i := range withGroups {
		if withGroups[i].Group == appName {
			out = append(out, resourceInputFromWG(withGroups[i]))
		}
	}
	return out
}

func resourceInputFromWG(wg derivation.ResourceWithGroup) derivation.ResourceInput {
	return derivation.ResourceInput{
		Namespace:       wg.Namespace,
		Kind:            wg.Kind,
		Name:            wg.Name,
		Labels:          wg.Labels,
		CreatedAt:       wg.CreatedAt,
		LastModifiedBy:  wg.LastModifiedBy,
		LastModifiedAt:  wg.LastModifiedAt,
		LastModifiedOp:  wg.LastModifiedOp,
		Images:          wg.Images,
		Ports:           wg.Ports,
		EnvVarKeys:      wg.EnvVarKeys,
		ConfigMapRefs:   wg.ConfigMapRefs,
		SecretRefs:      wg.SecretRefs,
		ServiceMappings: wg.ServiceMappings,
		IngressRules:    wg.IngressRules,
	}
}
