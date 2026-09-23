package validation

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/globalconfig"
	planseps "github.com/telark/rest/endpoints/plans"
)

// ValidationError marks a caller-fixable request so handlers can answer 400
// without string-matching the message.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func Invalid(msg string) error { return &ValidationError{Msg: msg} }

func Invalidf(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

func IsValidation(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

var (
	ErrInvalidScope = Invalid("scope.type must be applications or namespaces")
	ErrScopeUnion   = Invalid(
		"scope.type=applications requires applicationIds; scope.type=namespaces requires namespaces",
	)
	ErrInvalidTimeRange = Invalid("timeRange.endAt must be after timeRange.startAt")
	ErrPoliciesRequired = Invalid("at least one policy is required")
)

const (
	NameMaxLength        = 64
	DescriptionMaxLength = 512
	TaxonomyIDMaxLength  = 19
	TagIDsMax            = 20
	PriorityMin          = -100
	PriorityMax          = 100
)

const (
	fmtUnknownTemplate      = "unknown template id: %s"
	fmtTemplateScope        = "template %q does not support scope %q"
	fmtInvalidParams        = "template %q params invalid: %v"
	fmtInvalidName          = "name must be between 1 and %d characters"
	fmtInvalidDescription   = "description must be at most %d characters"
	fmtInvalidSeverity      = "severity must be one of %v"
	fmtInvalidMode          = "mode must be one of %v"
	fmtInvalidTimeMode      = "timeMode must be one of %v"
	fmtInvalidPriority      = "priority must be between %d and %d"
	fmtExcludedNamespaces   = "namespaces are excluded from discovery: %v"
	fmtMissingNamespaces    = "namespaces not found in cluster: %v"
	fmtDuplicateName        = "a protection plan named %q already exists"
	fmtInvalidEnvironmentID = "environmentID must be at most %d characters"
	fmtInvalidTagID         = "tagIDs[%d] must be at most %d characters"
	fmtTooManyTagIDs        = "tagIDs must have at most %d entries"
	msgDuplicateTagIDs      = "tagIDs must not contain duplicates"
)

var (
	allowedSeverities = []string{
		plans.SeverityLow, plans.SeverityMedium, plans.SeverityHigh, plans.SeverityCritical,
	}
	allowedModes     = []string{plans.ModeAudit, plans.ModeEnforce}
	allowedTimeModes = []string{plans.TimeModePermanent, plans.TimeModeTimeRange}
)

func PrepareRequest(req *planseps.PrepareProtectionPlanRequest) error {
	if err := Fields(req); err != nil {
		return err
	}
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

// Mirrors the CRD schema so a rejected plan never reaches the Kyverno deploy.
func Fields(req *planseps.PrepareProtectionPlanRequest) error {
	name := strings.TrimSpace(req.Name)
	if len(name) == constants.DefaultInitValue || len(name) > NameMaxLength {
		return Invalidf(fmtInvalidName, NameMaxLength)
	}
	if req.Description != nil && len(*req.Description) > DescriptionMaxLength {
		return Invalidf(fmtInvalidDescription, DescriptionMaxLength)
	}
	if !slices.Contains(allowedSeverities, req.Severity) {
		return Invalidf(fmtInvalidSeverity, allowedSeverities)
	}
	if !slices.Contains(allowedModes, req.Mode) {
		return Invalidf(fmtInvalidMode, allowedModes)
	}
	if !slices.Contains(allowedTimeModes, req.TimeMode) {
		return Invalidf(fmtInvalidTimeMode, allowedTimeModes)
	}
	if req.Priority < PriorityMin || req.Priority > PriorityMax {
		return Invalidf(fmtInvalidPriority, PriorityMin, PriorityMax)
	}
	return taxonomyFields(req)
}

func taxonomyFields(req *planseps.PrepareProtectionPlanRequest) error {
	if req.EnvironmentID != nil && len(*req.EnvironmentID) > TaxonomyIDMaxLength {
		return Invalidf(fmtInvalidEnvironmentID, TaxonomyIDMaxLength)
	}
	if len(req.TagIDs) > TagIDsMax {
		return Invalidf(fmtTooManyTagIDs, TagIDsMax)
	}
	for i, tag := range req.TagIDs {
		if len(tag) > TaxonomyIDMaxLength {
			return Invalidf(fmtInvalidTagID, i, TaxonomyIDMaxLength)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(req.TagIDs)))) != len(req.TagIDs) {
		return Invalid(msgDuplicateTagIDs)
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

type NamespaceLister func(ctx context.Context) ([]string, error)

// NamespaceScope fails closed: a namespace discovery ignores can never be covered
// by a plan, so it is rejected at the edge instead of silently protecting nothing.
// A namespace absent from the cluster is a caller error, not the transient apply
// failure it would otherwise surface as.
func NamespaceScope(ctx context.Context, scopeType string, namespaces []string, list NamespaceLister) error {
	if scopeType != plans.ScopeTypeNamespaces {
		return nil
	}
	if err := ExcludedNamespaces(namespaces, globalconfig.FetchExcludedNamespaces(ctx)); err != nil {
		return err
	}
	if list == nil {
		return nil
	}
	existing, err := list(ctx)
	if err != nil {
		return err
	}
	return MissingNamespaces(namespaces, existing)
}

func MissingNamespaces(namespaces, existing []string) error {
	missing := make([]string, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if !slices.Contains(existing, ns) {
			missing = append(missing, ns)
		}
	}
	if len(missing) > constants.DefaultInitValue {
		return Invalidf(fmtMissingNamespaces, missing)
	}
	return nil
}

func ExcludedNamespaces(namespaces, excluded []string) error {
	hits := make([]string, constants.DefaultInitValue, len(namespaces))
	for _, ns := range namespaces {
		if slices.Contains(excluded, ns) {
			hits = append(hits, ns)
		}
	}
	if len(hits) > constants.DefaultInitValue {
		return Invalidf(fmtExcludedNamespaces, hits)
	}
	return nil
}

func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func UniqueName(existing []plans.ProtectionPlan, name, excludeID string) error {
	wanted := NormalizeName(name)
	for i := range existing {
		if existing[i].ID == excludeID {
			continue
		}
		if NormalizeName(existing[i].Name) == wanted {
			return Invalidf(fmtDuplicateName, strings.TrimSpace(name))
		}
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
			return Invalidf(fmtUnknownTemplate, p.TemplateID)
		}
		if !TemplateSupports(tpl, scopeType) {
			return Invalidf(fmtTemplateScope, p.TemplateID, scopeType)
		}
		if err := plans.ValidateParams(tpl, p.Params); err != nil {
			return Invalidf(fmtInvalidParams, p.TemplateID, err)
		}
	}
	return nil
}

func TemplateSupports(tpl *plans.Template, scopeType string) bool {
	return slices.ContainsFunc(tpl.SupportedScopes, func(s plans.ScopeSupport) bool {
		return string(s) == scopeType
	})
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
