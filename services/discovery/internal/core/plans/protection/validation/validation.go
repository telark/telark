package validation

import (
	"errors"
	"fmt"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/constants"
	planseps "github.com/telark/rest/endpoints/plans"
)

var (
	ErrInvalidScope = errors.New("scope.type must be applications or namespaces")
	ErrScopeUnion   = errors.New(
		"scope.type=applications requires applicationIds; scope.type=namespaces requires namespaces")
	ErrInvalidTimeRange = errors.New("timeRange.endAt must be after timeRange.startAt")
	ErrPoliciesRequired = errors.New("at least one policy is required")
)

const (
	fmtUnknownTemplate = "unknown template id: %s"
	fmtTemplateScope   = "template %q does not support scope %q"
	fmtInvalidParams   = "template %q params invalid: %v"
)

func PrepareRequest(req *planseps.PrepareProtectionPlanRequest) error {
	if err := Scope(req.Scope); err != nil {
		return err
	}
	if err := Policies(req.Policies, req.Scope.Type); err != nil {
		return err
	}
	if req.TimeMode == plans.TimeModeTimeRange {
		return TimeRange(req.TimeRange)
	}
	return nil
}

func Scope(scope planseps.ScopeRequest) error {
	switch scope.Type {
	case plans.ScopeTypeApplications:
		appsEmpty := len(scope.ApplicationIDs) == constants.DefaultInitValue
		namespacesPresent := len(scope.Namespaces) > constants.DefaultInitValue
		if appsEmpty || namespacesPresent {
			return ErrScopeUnion
		}
	case plans.ScopeTypeNamespaces:
		namespacesEmpty := len(scope.Namespaces) == constants.DefaultInitValue
		appsPresent := len(scope.ApplicationIDs) > constants.DefaultInitValue
		if namespacesEmpty || appsPresent {
			return ErrScopeUnion
		}
	default:
		return ErrInvalidScope
	}
	return nil
}

func Policies(items []planseps.PolicyRequest, scopeType string) error {
	if len(items) == constants.DefaultInitValue {
		return ErrPoliciesRequired
	}
	for _, p := range items {
		tpl, ok := plans.GetTemplate(p.TemplateID)
		if !ok {
			return fmt.Errorf(fmtUnknownTemplate, p.TemplateID)
		}
		if !templateSupports(tpl, scopeType) {
			return fmt.Errorf(fmtTemplateScope, p.TemplateID, scopeType)
		}
		if err := plans.ValidateParams(tpl, p.Params); err != nil {
			return fmt.Errorf(fmtInvalidParams, p.TemplateID, err)
		}
	}
	return nil
}

func templateSupports(tpl *plans.Template, scopeType string) bool {
	for _, supported := range tpl.SupportedScopes {
		if string(supported) == scopeType {
			return true
		}
	}
	return false
}

func TimeRange(tr *planseps.TimeRangeRequest) error {
	if tr == nil {
		return ErrInvalidTimeRange
	}
	start, errStart := time.Parse(time.RFC3339, tr.StartAt)
	end, errEnd := time.Parse(time.RFC3339, tr.EndAt)
	if errStart != nil || errEnd != nil {
		return ErrInvalidTimeRange
	}
	if !end.After(start) {
		return ErrInvalidTimeRange
	}
	return nil
}
