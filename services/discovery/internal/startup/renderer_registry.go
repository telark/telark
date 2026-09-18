package startup

import (
	"fmt"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers all TemplateRenderers via init()
	"github.com/telark/discovery/internal/constants"
)

// Enforces a renderer per catalog entry, code uniqueness and the 63-byte K8s name budget;
// returns the first violation, or empty when all pass.
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
