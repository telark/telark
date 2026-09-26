package derivation

import (
	"cmp"
	"slices"
	"strings"

	"github.com/telark/discovery/internal/constants"
)

func GroupByWorkloadAnchor(resources []ResourceInput) []ResourceWithGroup {
	if len(resources) == constants.DefaultInitValue {
		return nil
	}

	resources = FilterNoise(resources)
	if len(resources) == constants.DefaultInitValue {
		return nil
	}
	identities := extractIdentity(resources)
	groups := buildGroups(resources, identities)
	groups = attachUnidentified(resources, identities, groups)
	groups = removeAnchorless(groups)
	out := flattenGroups(groups)
	slices.SortFunc(out, func(a, b ResourceWithGroup) int {
		return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Group, b.Group))
	})
	return out
}

func FilterNoise(resources []ResourceInput) []ResourceInput {
	out := make([]ResourceInput, constants.DefaultInitValue, len(resources))
	for i := range resources {
		r := &resources[i]
		if isNoiseResource(r) {
			continue
		}
		out = append(out, *r)
	}
	return out
}

func extractIdentity(resources []ResourceInput) []identityResult {
	results := make([]identityResult, len(resources))
	for i := range resources {
		r := &resources[i]
		results[i] = extractOneIdentity(r)
	}
	return results
}

func buildGroups(resources []ResourceInput, identities []identityResult) map[groupKey][]ResourceInput {
	groups := make(map[groupKey][]ResourceInput)
	for i := range resources {
		if !identities[i].identified {
			continue
		}
		key := groupKey{namespace: resources[i].Namespace, appKey: identities[i].appKey}
		groups[key] = append(groups[key], resources[i])
	}
	return groups
}

func attachUnidentified(
	resources []ResourceInput,
	identities []identityResult,
	groups map[groupKey][]ResourceInput,
) map[groupKey][]ResourceInput {
	byKey := make(map[string]int)
	for i := range resources {
		k := resourceMapKey(resources[i].Namespace, resources[i].Kind, resources[i].Name)
		byKey[k] = i
	}
	configIndex := referencedConfigIndex(groups)
	appKeysByNS := knownAppKeysByNamespace(groups)

	for i := range resources {
		if identities[i].identified {
			continue
		}
		r := &resources[i]
		if tryAttachViaConfigRef(r, configIndex, groups) {
			continue
		}
		attached := tryAttachViaOwnerRef(r, identities, byKey, resources, groups)
		if attached {
			continue
		}
		_ = tryAttachViaNameContains(r, groups, appKeysByNS)
	}
	return groups
}

func tryAttachViaOwnerRef(
	r *ResourceInput,
	identities []identityResult,
	byKey map[string]int,
	resources []ResourceInput,
	groups map[groupKey][]ResourceInput,
) bool {
	if len(r.OwnerReferences) == constants.DefaultInitValue {
		return false
	}
	rootIdx := resolveRootOwner(r, byKey, resources)
	if rootIdx == constants.DefaultReturnValue {
		return false
	}
	if !identities[rootIdx].identified {
		return false
	}
	root := &resources[rootIdx]
	key := groupKey{namespace: root.Namespace, appKey: identities[rootIdx].appKey}
	groups[key] = append(groups[key], *r)
	return true
}

func tryAttachViaNameContains(
	r *ResourceInput,
	groups map[groupKey][]ResourceInput,
	appKeysByNS map[string][]string,
) bool {
	keys := appKeysByNS[r.Namespace]
	if len(keys) == constants.DefaultInitValue {
		return false
	}
	var bestKey string
	bestLen := constants.DefaultInitValue
	for _, appKey := range keys {
		if !strings.Contains(r.Name, appKey) {
			continue
		}
		if len(appKey) > bestLen {
			bestLen = len(appKey)
			bestKey = appKey
		} else if len(appKey) == bestLen && (bestKey == constants.EmptyString || appKey < bestKey) {
			bestKey = appKey
		}
	}
	if bestKey == constants.EmptyString {
		return false
	}
	key := groupKey{namespace: r.Namespace, appKey: bestKey}
	groups[key] = append(groups[key], *r)
	return true
}

func removeAnchorless(groups map[groupKey][]ResourceInput) map[groupKey][]ResourceInput {
	for key, resources := range groups {
		n := countWorkloads(resources)
		if n == constants.DefaultInitValue {
			delete(groups, key)
		}
	}
	return groups
}

// referencedConfigIndex maps every ConfigMap and Secret a grouped workload reads
// to that workload's group. Groups are visited in a stable order so a config
// shared by two applications always lands in the same one.
func referencedConfigIndex(groups map[groupKey][]ResourceInput) map[string]groupKey {
	keys := make([]groupKey, constants.DefaultInitValue, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b groupKey) int {
		if c := strings.Compare(a.namespace, b.namespace); c != constants.DefaultInitValue {
			return c
		}
		return strings.Compare(a.appKey, b.appKey)
	})
	idx := make(map[string]groupKey)
	for _, key := range keys {
		for i := range groups[key] {
			r := &groups[key][i]
			if !workloadKinds[r.Kind] {
				continue
			}
			for _, n := range r.ConfigMapRefs {
				idx[resourceMapKey(r.Namespace, kindConfigMap, n)] = key
			}
			for _, n := range r.SecretRefs {
				idx[resourceMapKey(r.Namespace, kindSecret, n)] = key
			}
		}
	}
	return idx
}

func tryAttachViaConfigRef(r *ResourceInput, idx map[string]groupKey, groups map[groupKey][]ResourceInput) bool {
	if r.Kind != kindConfigMap && r.Kind != kindSecret {
		return false
	}
	key, ok := idx[resourceMapKey(r.Namespace, r.Kind, r.Name)]
	if !ok {
		return false
	}
	groups[key] = append(groups[key], *r)
	return true
}
