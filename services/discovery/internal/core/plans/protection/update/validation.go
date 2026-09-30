package update

import (
	"fmt"
	"strings"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
)

const fmtRawString = "%s"

// validatePolicies accumulates ALL policy errors instead of bailing on the first one — the UI
// surfaces them together so users fix the entire form in a single round-trip.
func validatePolicies(items []planseps.PolicyRequest, scopeType string) error {
	if len(items) == constants.DefaultInitValue {
		return validation.Invalidf(fmtRawString, ErrPoliciesRequired)
	}
	var errs []string
	for _, p := range items {
		tpl, ok := plans.GetTemplate(p.TemplateID)
		if !ok {
			errs = append(errs, fmt.Sprintf(string(ErrUnknownTemplate), p.TemplateID))
			continue
		}
		if !validation.TemplateSupports(tpl, scopeType) {
			errs = append(errs, fmt.Sprintf(string(ErrTemplateScope), p.TemplateID, scopeType))
			continue
		}
		if err := plans.ValidateParams(tpl, p.Params); err != nil {
			errs = append(errs, fmt.Sprintf(string(ErrInvalidParams), p.TemplateID, err))
		}
	}
	if err := validation.DuplicateTemplates(items); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > constants.DefaultInitValue {
		return validation.Invalidf(fmtRawString, strings.Join(errs, "; "))
	}
	return nil
}
