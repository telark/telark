package shared

import (
	"cmp"
	"slices"
	"strings"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
)

func ParseNamespaceList(raw string) []string {
	s := strings.TrimSpace(raw)
	if s == constants.EmptyString || strings.ToUpper(s) == constants.NamespaceAll {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, constants.DefaultInitValue, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != constants.EmptyString {
			out = append(out, v)
		}
	}
	return out
}

func BuildNamespaceItems(nsCounts map[string]int) []application.NamespaceEntry {
	items := make([]application.NamespaceEntry, constants.DefaultInitValue, len(nsCounts))
	for ns, count := range nsCounts {
		items = append(items, application.NamespaceEntry{Name: ns, ResourceCount: count})
	}
	slices.SortFunc(items, func(a, b application.NamespaceEntry) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return items
}
