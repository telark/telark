package shared

import (
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/discovery/derivation"
	kcoregroup "github.com/telark/kcore/resources/group"
)

func ToDerivationInputs(refs []kcoregroup.ResourceRef) []derivation.ResourceInput {
	out := make([]derivation.ResourceInput, constants.DefaultInitValue, len(refs))
	for i := range refs {
		r := &refs[i]
		out = append(out, derivation.ResourceInput{
			Namespace: r.Namespace,
			Kind:      r.Kind,
			Name:      r.Name,
			Labels:    r.Labels,
		})
	}
	return out
}
