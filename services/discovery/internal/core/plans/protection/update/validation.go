package update

import (
	"fmt"
	"strings"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	planseps "github.com/telark/rest/endpoints/plans"
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
	if len(errs) > constants.DefaultInitValue {
		return validation.Invalidf(fmtRawString, strings.Join(errs, "; "))
	}
	return nil
}

func validateTimeRange(tr *planseps.TimeRangeRequest) error {
	if tr == nil {
		return validation.Invalidf(fmtRawString, ErrInvalidTimeRange)
	}
	start, errStart := time.Parse(time.RFC3339, tr.StartAt)
	end, errEnd := time.Parse(time.RFC3339, tr.EndAt)
	if errStart != nil || errEnd != nil {
		return validation.Invalidf(fmtRawString, ErrInvalidTimeRange)
	}
	if !end.After(start) {
		return validation.Invalidf(fmtRawString, ErrInvalidTimeRange)
	}
	return nil
}
