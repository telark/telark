package shared

import (
	kcoregroup "github.com/telark/telark/internal/kcore/resources/group"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
)

func ToDerivationInputs(refs []kcoregroup.ResourceRef) []derivation.ResourceInput {
	out := make([]derivation.ResourceInput, constants.DefaultInitValue, len(refs))
	for i := range refs {
		r := &refs[i]
		owners := make([]derivation.OwnerReference, constants.DefaultInitValue, len(r.Owners))
		for j := range r.Owners {
			owners = append(owners, derivation.OwnerReference{Kind: r.Owners[j].Kind, Name: r.Owners[j].Name})
		}
		out = append(out, derivation.ResourceInput{
			Namespace:       r.Namespace,
			Kind:            r.Kind,
			Name:            r.Name,
			Labels:          r.Labels,
			OwnerReferences: owners,
			ConfigMapRefs:   r.ConfigMapRefs,
			SecretRefs:      r.SecretRefs,
		})
	}
	return out
}
