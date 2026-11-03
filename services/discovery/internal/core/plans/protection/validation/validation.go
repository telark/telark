package validation

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/plans"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/data/text"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
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
	fmtInvalidKindRef        = "kindRef must be at most %d characters"
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

	fmtUnknownParticipants = "participantRefs are not existing users: %v"
	fmtUsersUnavailable    = "users unavailable: %v"
)

var planNamePattern = regexp.MustCompile(constants.PlanNamePattern)

var (
	allowedSeverities = []string{
		plans.SeverityLow, plans.SeverityMedium, plans.SeverityHigh, plans.SeverityCritical,
	}
	allowedModes         = []string{plans.ModeAudit, plans.ModeEnforce}
	allowedTimeModes     = []string{plans.TimeModePermanent, plans.TimeModeTimeRange, plans.TimeModeRepeating}
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
	if req.TimeMode == plans.TimeModeRepeating {
		return Repeat(req.Repeat)
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
	if req.Repeat != nil {
		slices.SortStableFunc(req.Repeat.Days, func(a, b string) int {
			return cmp.Compare(slices.Index(plans.RepeatDays, a), slices.Index(plans.RepeatDays, b))
		})
	}
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
	if err := listFields(req); err != nil {
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

// The trimmed name and description are what gets stored.
func textFields(req *planseps.PrepareProtectionPlanRequest) error {
	if HasTemplateSyntax(req.Name) || (req.Description != nil && HasTemplateSyntax(*req.Description)) {
		return ErrTemplateSyntax
	}
	name, err := text.Check(req.Name, text.Rule{Field: constants.FieldPlanName, Min: constants.DefaultAddValue,
		Max: NameMaxLength, Pattern: planNamePattern})
	if err != nil {
		return Invalid(err.Error())
	}
	req.Name = name
	if req.Description != nil {
		description, err := text.Check(*req.Description, text.Rule{Field: constants.FieldPlanDescription,
			Max: DescriptionMaxLength, Multiline: true})
		if err != nil {
			return Invalid(err.Error())
		}
		req.Description = &description
	}
	return nil
}

// Kyverno substitutes {{ }} in the policy fields it renders; no user text may carry it.
func HasTemplateSyntax(value string) bool {
	return strings.Contains(value, templateOpen) || strings.Contains(value, templateClose)
}

func listFields(req *planseps.PrepareProtectionPlanRequest) error {
	counts := []struct {
		field        string
		count, limit int
	}{
		{constants.FieldParticipantRefs, len(req.ParticipantRefs), constants.PlanParticipantsMax},
		{constants.FieldScopeNamespaces, len(req.Scope.Namespaces), constants.PlanScopeTargetsMax},
		{constants.FieldScopeApplicationRefs, len(req.Scope.ApplicationRefs), constants.PlanScopeTargetsMax},
	}
	for _, c := range counts {
		if c.count > c.limit {
			return Invalidf(string(dataerrors.ErrTextTooManyItems), c.field, c.limit)
		}
	}
	return policyParams(req.Policies)
}

// List params (image patterns, tags) are stored trimmed; the template still checks each entry's pattern.
func policyParams(items []planseps.PolicyRequest) error {
	for _, p := range items {
		for key, value := range p.Params {
			values, ok := stringItems(value)
			if !ok {
				continue
			}
			checked, err := text.CheckList(values, constants.PlanParamItemsMax, text.Rule{
				Field: fmt.Sprintf(constants.FmtFieldPolicyParam, p.TemplateID, key),
				Min:   constants.DefaultAddValue, Max: constants.PlanParamItemMaxLen,
			})
			if err != nil {
				return Invalid(err.Error())
			}
			p.Params[key] = checked
		}
	}
	return nil
}

// A JSON body decodes a list as []any; a list holding a non-string is left to the template check.
func stringItems(value any) ([]string, bool) {
	switch list := value.(type) {
	case []string:
		return list, true
	case []any:
		values := make([]string, constants.DefaultInitValue, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			values = append(values, s)
		}
		return values, true
	default:
		return nil, false
	}
}

func taxonomyFields(req *planseps.PrepareProtectionPlanRequest) error {
	if req.EnvironmentRef != nil && len(*req.EnvironmentRef) > TaxonomyIDMaxLength {
		return Invalidf(fmtInvalidEnvironmentRef, TaxonomyIDMaxLength)
	}
	if req.KindRef != nil && len(*req.KindRef) > TaxonomyIDMaxLength {
		return Invalidf(fmtInvalidKindRef, TaxonomyIDMaxLength)
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

type UserLister func() ([]*userresource.User, error)

// Only refs not in kept are looked up, so a participant deleted since does not block every later edit. A nil
// lister (unit tests) skips the check; production always wires one.
func ParticipantRefs(refs, kept []string, list UserLister) error {
	added := slices.DeleteFunc(slices.Clone(refs), func(ref string) bool { return slices.Contains(kept, ref) })
	if len(added) == constants.DefaultInitValue || list == nil {
		return nil
	}
	users, err := list()
	if err != nil {
		return &UnavailableError{Msg: fmt.Sprintf(fmtUsersUnavailable, err)}
	}
	unknown := slices.DeleteFunc(added, func(ref string) bool { return isUser(users, ref) })
	if len(unknown) > constants.DefaultInitValue {
		return Invalidf(fmtUnknownParticipants, unknown)
	}
	return nil
}

// A copy keeps only the source's participants that are still users, rather than failing on one deleted since.
func KnownParticipants(refs []string, list UserLister) ([]string, error) {
	if len(refs) == constants.DefaultInitValue || list == nil {
		return refs, nil
	}
	users, err := list()
	if err != nil {
		return nil, &UnavailableError{Msg: fmt.Sprintf(fmtUsersUnavailable, err)}
	}
	return slices.DeleteFunc(slices.Clone(refs), func(ref string) bool { return !isUser(users, ref) }), nil
}

func isUser(users []*userresource.User, ref string) bool {
	return slices.ContainsFunc(users, func(u *userresource.User) bool { return u != nil && u.ID == ref })
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

func Repeat(r *plans.ProtectionPlanRepeat) error {
	if err := plans.ValidateRepeat(r); err != nil {
		return Invalid(err.Error())
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
