package notifications

import "github.com/telark/telark/services/exporter/internal/constants"

func DiffPtrStringSlices(oldVals []*string, newVals []*string) (added, removed []string) {
	return DiffStringSlices(Deref(oldVals), Deref(newVals))
}

func Deref(vals []*string) []string {
	out := make([]string, constants.DefaultInitValue, len(vals))
	for _, v := range vals {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

func DiffStringSlices(oldVals, newVals []string) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(oldVals))
	for _, v := range oldVals {
		oldSet[v] = struct{}{}
	}
	newSet := make(map[string]struct{}, len(newVals))
	for _, v := range newVals {
		newSet[v] = struct{}{}
	}
	for k := range newSet {
		if _, ok := oldSet[k]; !ok {
			added = append(added, k)
		}
	}
	for k := range oldSet {
		if _, ok := newSet[k]; !ok {
			removed = append(removed, k)
		}
	}
	return added, removed
}

// The user PATCH merge rewrites the body's refs to []*string before the notification reads it.
func ExtractNewRoleIDsFromBody(body map[string]any, key string) []*string {
	if ptrs, ok := body[key].([]*string); ok {
		return ptrs
	}
	ids := ExtractNewStringIDsFromBody(body, key)
	if ids == nil {
		return nil
	}
	out := make([]*string, constants.DefaultInitValue, len(ids))
	for _, s := range ids {
		out = append(out, &s)
	}
	return out
}

func ExtractNewStringIDsFromBody(body map[string]any, key string) []string {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, constants.DefaultInitValue, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}
