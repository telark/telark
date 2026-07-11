package notifications

import "github.com/telark/exporter/internal/constants"

func DiffPtrStringSlices(oldVals []*string, newVals []*string) (added, removed []string) {
	oldSet := make(map[string]struct{}, len(oldVals))
	for _, v := range oldVals {
		if v != nil {
			oldSet[*v] = struct{}{}
		}
	}
	newSet := make(map[string]struct{}, len(newVals))
	for _, v := range newVals {
		if v != nil {
			newSet[*v] = struct{}{}
		}
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

func ExtractNewRoleIDsFromBody(body map[string]any, key string) []*string {
	raw, ok := body[key]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]*string, constants.DefaultInitValue, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			continue
		}
		sCopy := s
		out = append(out, &sCopy)
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
