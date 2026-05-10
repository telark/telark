package policies

import (
	"fmt"

	"github.com/plsyro/data/plans"
)

const (
	maxK8sNameBytes      = 63
	planIDExampleLength  = 16
	scopeHashFixedLength = 8
)

// ValidateCatalog validates the template catalog against the renderer registry. It enforces:
//   - every catalog entry has a matching registered renderer
//   - every catalog entry declares a non-empty Code
//   - every Code is unique across the catalog
//   - every renderer's TemplateCode matches the catalog Code
//   - the maximum produced Kyverno Policy name fits in K8s 63-byte DNS label budget
//
// Returns the first violation found (empty string when all checks pass).
func ValidateCatalog() string {
	codes := map[string]string{}
	longestCode := ""
	for _, t := range plans.Templates {
		r, ok := GetRenderer(t.ID)
		if !ok {
			return fmt.Sprintf("policies: no renderer registered for template %q", t.ID)
		}
		if t.Code == "" {
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
	return ""
}

// projectedNameLength computes the worst-case length of PolicyName for a given template code.
// Format: "plsyro-" + planID + "-" + code + "-" + scopeHash. planID example length is 16
// matching the "pp-xxx-yyyy-zzzz" pattern produced by ids.GeneratePlanID.
func projectedNameLength(code string) int {
	return len("plsyro-") + planIDExampleLength + 1 + len(code) + 1 + scopeHashFixedLength
}
