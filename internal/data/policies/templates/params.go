package templates

import "fmt"

// paramStringSlice extracts a non-empty []string from a free-form params map.
func paramStringSlice(params map[string]any, key string) ([]string, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return nil, fmt.Errorf("param %q is required", key)
	}
	switch v := raw.(type) {
	case []string:
		if len(v) == 0 {
			return nil, fmt.Errorf("param %q must not be empty", key)
		}
		return append([]string(nil), v...), nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("param %q must be a string array", key)
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("param %q must not be empty", key)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("param %q must be a string array", key)
	}
}
