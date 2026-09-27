package derivation

import (
	"cmp"
	"slices"
	"strings"

	"github.com/telark/discovery/internal/constants"
)

func FirstGroupNameFromLabels(labels map[string]string) string {
	if labels == nil {
		return constants.EmptyString
	}
	for _, k := range GroupNameLabelKeys {
		if v := labels[k]; v != constants.EmptyString {
			return v
		}
	}
	return constants.EmptyString
}

func GroupNameOrFallback(name string, labels map[string]string) string {
	if v := FirstGroupNameFromLabels(labels); v != constants.EmptyString {
		return v
	}
	return name
}

func copyStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func copyIntSlice(s []int) []int {
	if s == nil {
		return []int{}
	}
	out := make([]int, len(s))
	copy(out, s)
	return out
}

func resourceMapKey(ns, kind, name string) string {
	return ns + resourceMapKeySeparator + kind + resourceMapKeySeparator + name
}

func isNoiseResource(r *ResourceInput) bool {
	if r.Name == noiseResourceKubeRootCaCrt || r.Name == noiseResourceDefault ||
		r.Name == noiseResourceKubernetes {
		return true
	}
	if r.Kind == kindSecret && strings.HasPrefix(r.Name, noiseResourceSecretHelmReleasePrefix) {
		return true
	}
	if r.Kind == kindJob && ownedByKind(r, kindCronJob) {
		return true
	}
	return false
}

func countWorkloads(resources []ResourceInput) int {
	n := constants.DefaultInitValue
	for i := range resources {
		if workloadKinds[resources[i].Kind] {
			n++
		}
	}
	return n
}

func knownAppKeysByNamespace(groups map[groupKey][]ResourceInput) map[string][]string {
	nsKeys := make(map[string][]string)
	for key := range groups {
		nsKeys[key.namespace] = append(nsKeys[key.namespace], key.appKey)
	}
	for ns, list := range nsKeys {
		slices.Sort(list)
		nsKeys[ns] = slices.Compact(list)
	}
	return nsKeys
}

func resolveRootOwner(r *ResourceInput, byKey map[string]int, resources []ResourceInput) int {
	visited := make(map[int]bool)
	idx := constants.DefaultReturnValue
	for _, ref := range r.OwnerReferences {
		k := resourceMapKey(r.Namespace, ref.Kind, ref.Name)
		if i, ok := byKey[k]; ok {
			idx = i
			break
		}
	}
	if idx == constants.DefaultReturnValue {
		return constants.DefaultReturnValue
	}
	for idx >= constants.DefaultInitValue && !visited[idx] {
		visited[idx] = true
		next := constants.DefaultReturnValue
		res := &resources[idx]
		for _, ref := range res.OwnerReferences {
			k := resourceMapKey(res.Namespace, ref.Kind, ref.Name)
			if i, ok := byKey[k]; ok {
				next = i
				break
			}
		}
		if next == constants.DefaultReturnValue {
			return idx
		}
		idx = next
	}
	return idx
}

func extractOneIdentity(r *ResourceInput) identityResult {
	if r.Labels == nil {
		return identityResult{signal: signalUnidentified, identified: false}
	}
	if v := r.Labels[labelAppName]; v != constants.EmptyString {
		return identityResult{appKey: appKeyFromLabel(v), signal: signalAppName, identified: true}
	}
	if v := r.Labels[labelPartOf]; v != constants.EmptyString {
		return identityResult{appKey: appKeyFromLabel(v), signal: signalPartOf, identified: true}
	}
	comp := r.Labels[labelComponent]
	inst := r.Labels[labelInstance]
	if comp != constants.EmptyString && inst != constants.EmptyString {
		key := inst
		if strings.HasSuffix(inst, releaseSuffix) {
			key = strings.TrimSuffix(inst, releaseSuffix)
		}
		return identityResult{appKey: appKeyFromLabel(key), signal: signalComponent, identified: true}
	}
	if v := r.Labels[labelAppLegacy]; v != constants.EmptyString {
		return identityResult{appKey: appKeyFromLabel(v), signal: signalAppLegacy, identified: true}
	}
	return identityResult{signal: signalUnidentified, identified: false}
}

func appKeyFromLabel(value string) string {
	return strings.ToLower(strings.ReplaceAll(value, appKeyUnderscore, appKeyDash))
}

func AppKey(labels map[string]string) string {
	return extractOneIdentity(&ResourceInput{Labels: labels}).appKey
}

func flattenGroups(groups map[groupKey][]ResourceInput) []ResourceWithGroup {
	keys := make([]groupKey, constants.DefaultInitValue, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b groupKey) int {
		if c := cmp.Compare(a.namespace, b.namespace); c != 0 {
			return c
		}
		return cmp.Compare(a.appKey, b.appKey)
	})
	var out []ResourceWithGroup
	for _, key := range keys {
		for _, r := range groups[key] {
			out = append(out, withGroup(r, key.appKey))
		}
	}
	return out
}

func withGroup(r ResourceInput, group string) ResourceWithGroup {
	return ResourceWithGroup{
		Namespace:       r.Namespace,
		Kind:            r.Kind,
		Name:            r.Name,
		Labels:          r.Labels,
		Group:           group,
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
	}
}

func ownedByKind(r *ResourceInput, kind string) bool {
	return slices.ContainsFunc(r.OwnerReferences, func(o OwnerReference) bool { return o.Kind == kind })
}
