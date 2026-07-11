package startup

import (
	"fmt"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers all TemplateRenderers via init()
	"github.com/telark/discovery/constants"
)

// ValidateRendererRegistry returns the first catalog or registry violation, or empty string when
// all checks pass. It enforces presence of a renderer per catalog entry, code uniqueness, and the
// 63-byte K8s name budget.
func ValidateRendererRegistry() string {
	for _, t := range plans.Templates {
		if _, ok := policies.GetRenderer(t.ID); !ok {
			return fmt.Sprintf(string(constants.ErrRendererNotRegistered), t.ID)
		}
	}
	if msg := policies.ValidateCatalog(); msg != constants.EmptyString {
		return msg
	}
	return constants.EmptyString
}
