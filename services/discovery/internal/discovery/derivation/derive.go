package derivation

import (
	"strings"

	"github.com/telark/discovery/internal/constants"
)

func DeriveGroups(resources []ResourceInput) ([]ResourceWithGroup, []string) {
	if len(resources) == constants.DefaultInitValue {
		return nil, nil
	}
	counts := tokenCounts(resources)
	repeated := repeatedSet(len(resources), counts)
	seen := make(map[string]bool)
	var groups []string
	out := make([]ResourceWithGroup, constants.DefaultInitValue, len(resources))
	for _, r := range resources {
		g := groupForResource(r, repeated)
		out = append(out, ResourceWithGroup{
			Namespace:       r.Namespace,
			Kind:            r.Kind,
			Name:            r.Name,
			Labels:          r.Labels,
			Group:           g,
			CreatedAt:       r.CreatedAt,
			LastModifiedBy:  r.LastModifiedBy,
			LastModifiedAt:  r.LastModifiedAt,
			LastModifiedOp:  r.LastModifiedOp,
			Images:          copyStringSlice(r.Images),
			Ports:           copyIntSlice(r.Ports),
			EnvVarKeys:      copyStringSlice(r.EnvVarKeys),
			ConfigMapRefs:   copyStringSlice(r.ConfigMapRefs),
			SecretRefs:      copyStringSlice(r.SecretRefs),
			ServiceMappings: copyStringSlice(r.ServiceMappings),
			IngressRules:    copyStringSlice(r.IngressRules),
		})
		if !seen[g] {
			seen[g] = true
			groups = append(groups, g)
		}
	}
	return out, groups
}

func groupForResource(r ResourceInput, repeated map[string]bool) string {
	segments := tokenize(GroupNameOrFallback(r.Name, r.Labels))
	unique := stripRepeated(segments, repeated)
	if len(unique) == constants.DefaultInitValue {
		return fallbackGroup
	}
	return strings.Join(unique, "-")
}

func stripRepeated(segments []string, repeated map[string]bool) []string {
	var out []string
	for _, s := range segments {
		if !repeated[s] {
			out = append(out, s)
		}
	}
	return out
}
