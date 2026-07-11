package update

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
)

type PolicyKey struct {
	TemplateID string
	ParamsHash string
}

func policyKey(p plans.ProtectionPlanPolicy) PolicyKey {
	return PolicyKey{TemplateID: p.TemplateID, ParamsHash: hashParams(p.Params)}
}

func hashParams(params map[string]any) string {
	if len(params) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	keys := make([]string, constants.DefaultInitValue, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	ordered := make([][2]any, constants.DefaultInitValue, len(keys))
	for _, k := range keys {
		ordered = append(ordered, [2]any{k, normalizeParamValue(params[k])})
	}
	bytes, err := json.Marshal(ordered)
	if err != nil {
		return constants.EmptyString
	}
	sum := sha256.Sum256(bytes)
	return hex.EncodeToString(sum[:])
}

func normalizeParamValue(v any) any {
	switch cast := v.(type) {
	case []any:
		out := make([]string, constants.DefaultInitValue, len(cast))
		for _, item := range cast {
			s, ok := item.(string)
			if !ok {
				return cast
			}
			out = append(out, s)
		}
		slices.Sort(out)
		return out
	case []string:
		dup := append([]string(nil), cast...)
		slices.Sort(dup)
		return dup
	default:
		return v
	}
}

// PolicyDiff captures added/removed/unchanged policies between two policy lists.
type PolicyDiff struct {
	Added     []plans.ProtectionPlanPolicy
	Removed   []plans.ProtectionPlanPolicy
	Unchanged []plans.ProtectionPlanPolicy
}

// TargetDiff captures added/removed/unchanged scope targets between two target lists.
type TargetDiff struct {
	Added     []string
	Removed   []string
	Unchanged []string
}

func diffPolicies(oldList, newList []plans.ProtectionPlanPolicy) PolicyDiff {
	oldKeys := make(map[PolicyKey]plans.ProtectionPlanPolicy, len(oldList))
	for _, p := range oldList {
		oldKeys[policyKey(p)] = p
	}
	newKeys := make(map[PolicyKey]plans.ProtectionPlanPolicy, len(newList))
	for _, p := range newList {
		newKeys[policyKey(p)] = p
	}

	diff := PolicyDiff{}
	for k, p := range newKeys {
		if _, ok := oldKeys[k]; ok {
			diff.Unchanged = append(diff.Unchanged, p)
		} else {
			diff.Added = append(diff.Added, p)
		}
	}
	for k, p := range oldKeys {
		if _, ok := newKeys[k]; !ok {
			diff.Removed = append(diff.Removed, p)
		}
	}
	return diff
}

func diffTargets(oldList, newList []string) TargetDiff {
	oldSet := stringSet(oldList)
	newSet := stringSet(newList)
	diff := TargetDiff{}
	for v := range newSet {
		if _, ok := oldSet[v]; ok {
			diff.Unchanged = append(diff.Unchanged, v)
		} else {
			diff.Added = append(diff.Added, v)
		}
	}
	for v := range oldSet {
		if _, ok := newSet[v]; !ok {
			diff.Removed = append(diff.Removed, v)
		}
	}
	slices.Sort(diff.Added)
	slices.Sort(diff.Removed)
	slices.Sort(diff.Unchanged)
	return diff
}

func stringSet(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		out[item] = struct{}{}
	}
	return out
}
