package policies

import (
	"fmt"

	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
)

const (
	maxK8sNameBytes        = 63
	planIDExampleLength    = 16
	separatorLengthSegment = 1
)

// Returns the first violation found, or an empty string when the catalog is consistent.
func ValidateCatalog() string {
	codes := map[string]string{}
	longestCode := constants.EmptyString
	for _, t := range plans.Templates {
		r, ok := GetRenderer(t.ID)
		if !ok {
			return fmt.Sprintf("policies: no renderer registered for template %q", t.ID)
		}
		if t.Code == constants.EmptyString {
			return fmt.Sprintf("policies: template %q has empty Code", t.ID)
		}
		if other, exists := codes[t.Code]; exists {
			return fmt.Sprintf("policies: template %q Code %q collides with template %q", t.ID, t.Code, other)
		}
		codes[t.Code] = t.ID
		if r.TemplateCode() != t.Code {
			return fmt.Sprintf(
				"policies: template %q Code mismatch: catalog=%q renderer=%q",
				t.ID, t.Code, r.TemplateCode(),
			)
		}
		if len(t.Code) > len(longestCode) {
			longestCode = t.Code
		}
	}
	if got := projectedNameLength(longestCode); got > maxK8sNameBytes {
		return fmt.Sprintf(
			"policies: max projected Policy name length %d exceeds K8s limit %d (longest code=%q)",
			got, maxK8sNameBytes, longestCode,
		)
	}
	return constants.EmptyString
}

func projectedNameLength(code string) int {
	return len("telark-") + planIDExampleLength + separatorLengthSegment + len(code) + separatorLengthSegment + scopeHashLength
}
