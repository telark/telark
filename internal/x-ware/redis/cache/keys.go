package cache

import "strings"

const KeyDelimiter = ":"

func BuildKey(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			filtered = append(filtered, s)
		}
	}
	return strings.Join(filtered, KeyDelimiter)
}

func GenerateKey(action string, resourceType string, name string) string {
	return BuildKey(strings.ToLower(action), resourceType, name)
}

func ValidateKey(key string) bool {
	return strings.TrimSpace(key) != ""
}
