package derivation

import (
	"strings"

	"github.com/telark/telark/services/discovery/internal/constants"
)

func tokenize(s string) []string {
	if s == constants.EmptyString {
		return nil
	}
	lower := strings.ToLower(s)
	var out []string
	for _, part := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == '-' || r == '.'
	}) {
		if part != constants.EmptyString {
			out = append(out, part)
		}
	}
	return out
}

func segmentsForResource(r ResourceInput) []string {
	seen := make(map[string]bool)
	var out []string
	for _, seg := range tokenize(r.Name) {
		if !seen[seg] {
			seen[seg] = true
			out = append(out, seg)
		}
	}
	for _, seg := range tokenize(FirstGroupNameFromLabels(r.Labels)) {
		if !seen[seg] {
			seen[seg] = true
			out = append(out, seg)
		}
	}
	return out
}

func tokenCounts(resources []ResourceInput) map[string]int {
	counts := make(map[string]int)
	for _, r := range resources {
		for _, seg := range segmentsForResource(r) {
			counts[seg]++
		}
	}
	return counts
}

func repeatedSet(n int, counts map[string]int) map[string]bool {
	if n == constants.DefaultInitValue {
		return nil
	}
	threshold := max(int(float64(n)*repeatedRatio), minRepeatedNum)
	set := make(map[string]bool)
	for tok, c := range counts {
		if c >= threshold {
			set[tok] = true
		}
	}
	return set
}
