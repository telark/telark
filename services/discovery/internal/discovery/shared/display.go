package shared

import (
	"slices"
	"strings"

	"github.com/telark/discovery/internal/constants"
)

func BuildDisplayName(groupKey, namespace string) string {
	original := groupKey
	if namespace != constants.EmptyString && strings.HasPrefix(groupKey, namespace+"-") {
		groupKey = groupKey[len(namespace)+constants.DefaultAddValue:]
	}
	groupKey = strings.ReplaceAll(groupKey, "-", " ")
	groupKey = strings.ReplaceAll(groupKey, "_", " ")
	groupKey = strings.TrimSpace(groupKey)
	groupKey = titleCase(groupKey)
	groupKey = restoreAcronyms(groupKey)
	if groupKey == constants.EmptyString {
		return original
	}
	return groupKey
}

func titleCase(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == constants.EmptyString {
			continue
		}
		runes := []rune(p)
		if len(runes) > constants.DefaultInitValue {
			runes[constants.DefaultInitValue] = toUpper(runes[constants.DefaultInitValue])
			for j := range len(runes) - constants.DefaultAddValue {
				runes[j+constants.DefaultAddValue] = toLower(runes[j+constants.DefaultAddValue])
			}
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, constants.SpaceSeparator)
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}

func toLower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

func restoreAcronyms(s string) string {
	parts := strings.Fields(s)
	for i, w := range parts {
		lower := strings.ToLower(w)
		if repl, ok := constants.AcronymMap[lower]; ok {
			parts[i] = repl
		}
	}
	return strings.Join(parts, constants.SpaceSeparator)
}

func PrimaryNamespaceFromCounts(nsCounts map[string]int) string {
	if len(nsCounts) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	keys := make([]string, constants.DefaultInitValue, len(nsCounts))
	for k := range nsCounts {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys[constants.DefaultInitValue]
}

func ToDisplayName(name string) string {
	return BuildDisplayName(name, constants.EmptyString)
}
