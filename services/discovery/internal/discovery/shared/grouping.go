package shared

import "github.com/telark/telark/services/discovery/internal/discovery/derivation"

func OrderedGroupNames(withGroups []derivation.ResourceWithGroup) []string {
	seen := make(map[string]bool)
	var order []string
	for _, r := range withGroups {
		if !seen[r.Group] {
			seen[r.Group] = true
			order = append(order, r.Group)
		}
	}
	return order
}
