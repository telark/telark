package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/telark/data/plans"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/helpers/telarkconfig"
	planseps "github.com/telark/rest/endpoints/plans"
	xauthz "github.com/telark/x-ware/authz"
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

// ConflictError marks a request that clashes with existing state (a taken name), which is a
// 409, not a malformed request.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

func IsConflict(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}

// UnavailableError marks a check that could not load what it compares against; it fails closed
// with a 503 instead of passing vacuously.
type UnavailableError struct{ Msg string }

func (e *UnavailableError) Error() string { return e.Msg }

func IsUnavailable(err error) bool {
	var target *UnavailableError
	return errors.As(err, &target)
}

type ForbiddenError struct{ Msg string }

func (e *ForbiddenError) Error() string { return e.Msg }

func IsForbidden(err error) bool {
	var target *ForbiddenError
	return errors.As(err, &target)
}

var (
	ErrInvalidScope = Invalid("scope.type must be applications or namespaces")
	ErrScopeUnion   = Invalid(
		"scope.type=applications requires applicationRefs; scope.type=namespaces requires namespaces",
	)
	ErrInvalidTimeRange  = Invalid("timeRange.endAt must be after timeRange.startAt")
	ErrTimeRangeRequired = Invalid("timeRange is required when timeMode is time_range")
	ErrTimeRangeFormat   = Invalid("timeRange.startAt and timeRange.endAt must be RFC3339 timestamps")
	ErrTimeRangeElapsed  = Invalid("timeRange.endAt is in the past; the plan window has already ended")
	ErrPoliciesRequired  = Invalid("at least one policy is required")

	ErrTemplateSyntax    = Invalid("name and description must not contain '{{' or '}}'")
	ErrEnforceNeedsOwner = &ForbiddenError{
		Msg: "enforce mode on a namespaces scope requires the Owner level on protection plans; use audit mode",
	}

	ErrExclusionResourcesScope  = Invalid("scope.exclusions.resources is only allowed when scope.type=applications")
	ErrExclusionKindInvalid     = Invalid("scope.exclusions.kinds entries must be non-empty base kinds without '/'")
	ErrExclusionResourceInvalid = Invalid("scope.exclusions.resources entries require a valid kind (no subresource), " +
		"name and namespace within their length limits")
)

const (
	NameMaxLength        = 64
	DescriptionMaxLength = 512
	TaxonomyIDMaxLength  = 19
	TagRefsMax           = 20
	PriorityMin          = -100
	PriorityMax          = 100
)

const (
	fmtUnknownTemplate       = "unknown template id: %s"
	fmtTemplateScope         = "template %q does not support scope %q"
	fmtInvalidParams         = "template %q params invalid: %v"
	fmtInvalidName           = "name must be between 1 and %d characters"
	fmtInvalidDescription    = "description must be at most %d characters"
	fmtInvalidSeverity       = "severity must be one of %v"
	fmtInvalidMode           = "mode must be one of %v"
	fmtInvalidTimeMode       = "timeMode must be one of %v"
	fmtInvalidApprovalMode   = "approvalMode must be one of %v"
	fmtInvalidPriority       = "priority must be between %d and %d"
	fmtExcludedNamespaces    = "namespaces are excluded from discovery: %v"
	fmtPlatformNamespaces    = "namespaces are reserved by the platform and ignored by the policy engine: %v"
	fmtDuplicateTemplate     = "template %q is listed more than once with different params"
	fmtMissingNamespaces     = "namespaces not found in cluster: %v"
	fmtDuplicateName         = "a protection plan named %q already exists"
	fmtInvalidEnvironmentRef = "environmentRef must be at most %d characters"
	fmtInvalidTagID          = "tagRefs[%d] must be at most %d characters"
	fmtTooManyTagRefs        = "tagRefs must have at most %d entries"
	msgDuplicateTagRefs      = "tagRefs must not contain duplicates"

	fmtUnknownEnvironmentRef = "environmentRef %q is not a known plan environment"
	fmtExcludedUnavailable   = "excluded namespaces unavailable: %v"
	fmtEnvironmentsUnavail   = "plan environments unavailable: %v"
	templateOpen             = "{{"
	templateClose            = "}}"

	fmtTooManyExclusionKinds     = "scope.exclusions.kinds: at most %d"
	fmtTooManyExclusionResources = "scope.exclusions.resources: at most %d"

	policyKeySep = "\x00"
)

var (
	allowedSeverities = []string{
		plans.SeverityLow, plans.SeverityMedium, plans.SeverityHigh, plans.SeverityCritical,
	}
	allowedModes         = []string{plans.ModeAudit, plans.ModeEnforce}
	allowedTimeModes     = []string{plans.TimeModePermanent, plans.TimeModeTimeRange}
	allowedApprovalModes = []string{plans.ApprovalModeAutomatic, plans.ApprovalModeRequired}
)

func PrepareRequest(req *planseps.PrepareProtectionPlanRequest, now time.Time) error {
	Normalize(req)
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
		return TimeRangeOpen(req.TimeRange, now)
	}
	return nil
}

// Repeated targets or identical policies would render the same policy name twice.
func Normalize(req *planseps.PrepareProtectionPlanRequest) {
	req.Scope.Namespaces = dedupe(req.Scope.Namespaces)
	req.Scope.ApplicationRefs = dedupe(req.Scope.ApplicationRefs)
	seen := map[string]struct{}{}
	req.Policies = slices.DeleteFunc(req.Policies, func(p planseps.PolicyRequest) bool {
		key := p.TemplateID + policyKeySep + paramsKey(p.Params)
		if _, dup := seen[key]; dup {
			return true
		}
		seen[key] = struct{}{}
		return false
	})
}

func dedupe(items []string) []string {
	if items == nil {
		return nil
	}
	return slices.Compact(slices.Sorted(slices.Values(items)))
}

func paramsKey(params map[string]any) string {
	raw, err := json.Marshal(params)
	if err != nil {
		return constants.EmptyString
	}
	return string(raw)
}

// Mirrors the CRD schema so a rejected plan never reaches the Kyverno deploy.
func Fields(req *planseps.PrepareProtectionPlanRequest) error {
	if err := textFields(req); err != nil {
		return err
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
	if req.ApprovalMode != nil && !slices.Contains(allowedApprovalModes, *req.ApprovalMode) {
		return Invalidf(fmtInvalidApprovalMode, allowedApprovalModes)
	}
	return taxonomyFields(req)
}

func textFields(req *planseps.PrepareProtectionPlanRequest) error {
	name := strings.TrimSpace(req.Name)
	if len(name) == constants.DefaultInitValue || len(name) > NameMaxLength {
		return Invalidf(fmtInvalidName, NameMaxLength)
	}
	if req.Description != nil && len(*req.Description) > DescriptionMaxLength {
		return Invalidf(fmtInvalidDescription, DescriptionMaxLength)
	}
	if HasTemplateSyntax(req.Name) || (req.Description != nil && HasTemplateSyntax(*req.Description)) {
		return ErrTemplateSyntax
	}
	return nil
}

// Kyverno substitutes {{ }} in the policy fields it renders; no user text may carry it.
func HasTemplateSyntax(value string) bool {
	return strings.Contains(value, templateOpen) || strings.Contains(value, templateClose)
}

func taxonomyFields(req *planseps.PrepareProtectionPlanRequest) error {
	if req.EnvironmentRef != nil && len(*req.EnvironmentRef) > TaxonomyIDMaxLength {
		return Invalidf(fmtInvalidEnvironmentRef, TaxonomyIDMaxLength)
	}
	if len(req.TagRefs) > TagRefsMax {
		return Invalidf(fmtTooManyTagRefs, TagRefsMax)
	}
	for i, tag := range req.TagRefs {
		if len(tag) > TaxonomyIDMaxLength {
			return Invalidf(fmtInvalidTagID, i, TaxonomyIDMaxLength)
		}
	}
	if len(slices.Compact(slices.Sorted(slices.Values(req.TagRefs)))) != len(req.TagRefs) {
		return Invalid(msgDuplicateTagRefs)
	}
	return nil
}

func Scope(scope planseps.ScopeRequest) error {
	switch scope.Type {
	case plans.ScopeTypeApplications:
		appsEmpty := len(scope.ApplicationRefs) == constants.DefaultInitValue
		namespacesPresent := len(scope.Namespaces) > constants.DefaultInitValue
		if appsEmpty || namespacesPresent {
			return ErrScopeUnion
		}
	case plans.ScopeTypeNamespaces:
		namespacesEmpty := len(scope.Namespaces) == constants.DefaultInitValue
		appsPresent := len(scope.ApplicationRefs) > constants.DefaultInitValue
		if namespacesEmpty || appsPresent {
			return ErrScopeUnion
		}
	default:
		return ErrInvalidScope
	}
	return Exclusions(scope)
}

func Exclusions(scope planseps.ScopeRequest) error {
	e := scope.Exclusions
	if e == nil {
		return nil
	}
	if len(e.Kinds) > plans.ExclusionKindsMax {
		return Invalidf(fmtTooManyExclusionKinds, plans.ExclusionKindsMax)
	}
	if slices.ContainsFunc(e.Kinds, invalidExclusionKind) {
		return ErrExclusionKindInvalid
	}
	if len(e.Resources) > plans.ExclusionResourcesMax {
		return Invalidf(fmtTooManyExclusionResources, plans.ExclusionResourcesMax)
	}
	if slices.ContainsFunc(e.Resources, invalidExcludedResource) {
		return ErrExclusionResourceInvalid
	}
	if scope.Type == plans.ScopeTypeNamespaces && len(e.Resources) > constants.DefaultInitValue {
		return ErrExclusionResourcesScope
	}
	return nil
}

func invalidExclusionKind(kind string) bool {
	return strings.TrimSpace(kind) == constants.EmptyString ||
		len(kind) > plans.ExclusionKindMaxLength ||
		strings.Contains(kind, plans.SubresourceSeparator)
}

func invalidExcludedResource(r plans.ProtectionPlanExcludedResource) bool {
	return invalidExclusionKind(r.Kind) ||
		strings.TrimSpace(r.Name) == constants.EmptyString ||
		strings.TrimSpace(r.Namespace) == constants.EmptyString ||
		len(r.Name) > plans.ExclusionNameMaxLength ||
		len(r.Namespace) > plans.ExclusionNamespaceMaxLength
}

type NamespaceLister func(ctx context.Context) ([]string, error)

// Fails closed: an excluded namespace can never be covered, and a missing one is a caller error,
// so both are rejected at the edge instead of silently protecting nothing or failing at apply.
func NamespaceScope(ctx context.Context, scopeType string, namespaces []string, list NamespaceLister) error {
	if scopeType != plans.ScopeTypeNamespaces {
		return nil
	}
	excluded, err := telarkconfig.ExcludedNamespaces(ctx)
	if err != nil {
		return &UnavailableError{Msg: fmt.Sprintf(fmtExcludedUnavailable, err)}
	}
	// Platform first: with self-monitoring off the release namespace is excluded too, and the
	// reserved-namespace message is the one that tells the caller why.
	if err := PlatformNamespaces(namespaces, telarkconfig.OwnNamespace()); err != nil {
		return err
	}
	if err := ExcludedNamespaces(namespaces, excluded); err != nil {
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
	missing := slices.DeleteFunc(slices.Clone(namespaces), func(ns string) bool {
		return slices.Contains(existing, ns)
	})
	if len(missing) > constants.DefaultInitValue {
		return Invalidf(fmtMissingNamespaces, missing)
	}
	return nil
}

func ExcludedNamespaces(namespaces, excluded []string) error {
	hits := slices.DeleteFunc(slices.Clone(namespaces), func(ns string) bool {
		return !slices.Contains(excluded, ns)
	})
	if len(hits) > constants.DefaultInitValue {
		return Invalidf(fmtExcludedNamespaces, hits)
	}
	return nil
}

func PlatformNamespaces(namespaces []string, own string) error {
	if own != constants.EmptyString && slices.Contains(namespaces, own) {
		return Invalidf(fmtPlatformNamespaces, []string{own})
	}
	return nil
}

// Namespaces the policy engine never evaluates: a plan targeting them would look healthy while
// enforcing nothing.
func IgnoredNamespaces(ctx context.Context) ([]string, error) {
	excluded, err := telarkconfig.ExcludedNamespaces(ctx)
	if err != nil {
		return nil, &UnavailableError{Msg: fmt.Sprintf(fmtExcludedUnavailable, err)}
	}
	ignored := slices.Clone(excluded)
	if own := telarkconfig.OwnNamespace(); own != constants.EmptyString && !slices.Contains(ignored, own) {
		ignored = append(ignored, own)
	}
	return ignored, nil
}

// Internal peers pass, as they do in the route middleware.
func CallerOwnsPlans(ctx context.Context) bool {
	identity, ok := xauthz.FromContext(ctx)
	return ok && (identity.Internal || xauthz.Allows(identity, xauthz.Own(roledata.ScopeProtectionPlans)))
}

// An enforce plan on a namespace freezes every workload in it, not only the caller's own.
func EnforceScope(ctx context.Context, scopeType, mode string) error {
	if scopeType == plans.ScopeTypeNamespaces && mode == plans.ModeEnforce && !CallerOwnsPlans(ctx) {
		return ErrEnforceNeedsOwner
	}
	return nil
}

type EnvironmentLister func() ([]string, error)

// A nil lister (standalone bootstrap, unit tests) skips the check; production always wires one.
func EnvironmentRef(environmentRef *string, list EnvironmentLister) error {
	if environmentRef == nil || *environmentRef == constants.EmptyString || list == nil {
		return nil
	}
	known, err := list()
	if err != nil {
		return &UnavailableError{Msg: fmt.Sprintf(fmtEnvironmentsUnavail, err)}
	}
	if !slices.Contains(known, *environmentRef) {
		return Invalidf(fmtUnknownEnvironmentRef, *environmentRef)
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
			return &ConflictError{Msg: fmt.Sprintf(fmtDuplicateName, strings.TrimSpace(name))}
		}
	}
	return nil
}

func Policies(items []planseps.PolicyRequest, scopeType string) error {
	if len(items) == constants.DefaultInitValue {
		return ErrPoliciesRequired
	}
	if err := DuplicateTemplates(items); err != nil {
		return err
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

// Params are not part of the policy name, so two entries of one template would render the same
// name and the second would silently overwrite the first.
func DuplicateTemplates(items []planseps.PolicyRequest) error {
	for i, p := range items {
		if slices.ContainsFunc(items[:i], func(o planseps.PolicyRequest) bool { return o.TemplateID == p.TemplateID }) {
			return Invalidf(fmtDuplicateTemplate, p.TemplateID)
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
		return ErrTimeRangeRequired
	}
	start, errStart := time.Parse(time.RFC3339, tr.StartAt)
	end, errEnd := time.Parse(time.RFC3339, tr.EndAt)
	if errStart != nil || errEnd != nil {
		return ErrTimeRangeFormat
	}
	if !end.After(start) {
		return ErrInvalidTimeRange
	}
	return nil
}

// A window that has already ended would activate and terminate on the next tick.
func TimeRangeOpen(tr *planseps.TimeRangeRequest, now time.Time) error {
	if err := TimeRange(tr); err != nil {
		return err
	}
	end, _ := time.Parse(time.RFC3339, tr.EndAt)
	if !end.After(now) {
		return ErrTimeRangeElapsed
	}
	return nil
}
