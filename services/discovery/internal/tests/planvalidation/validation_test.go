package planvalidation

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	taxonomyEnvironmentRef = "cat-00002-0001-0001"
	taxonomyTagID          = "cat-00003-0001-0001"
	taxonomyTagB           = "cat-00003-0001-0002"
	validStartAt           = "2026-01-01T00:00:00Z"
	applicationID          = "a1"
	crossedNamespace       = "n"
	validCase              = "valid"
	appNamespace           = "app"
	prodGuardName          = "Prod Guard"
	prodGuardID            = "pp-1"
	validEndAt             = "2026-01-02T00:00:00Z"
	kubeSystemNamespace    = "kube-system"
	excludedKind           = "ConfigMap"
	excludedName           = "wa1"
	subresourceKind        = "Deployment/scale"
	tplBlockCreate         = "block-create"
	tplBlockImageTags      = "block-image-tags"
	paramTags              = "tags"
	tagLatest              = "latest"
	ownNamespace           = "telark"
)

var (
	beforeWindow = time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	afterWindow  = time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)
)

func strptr(s string) *string { return &s }

func excludedResource() plans.ProtectionPlanExcludedResource {
	return plans.ProtectionPlanExcludedResource{Kind: excludedKind, Name: excludedName, Namespace: appNamespace}
}

func resourceExclusions() *plans.ProtectionPlanScopeExclusions {
	return &plans.ProtectionPlanScopeExclusions{Resources: []plans.ProtectionPlanExcludedResource{excludedResource()}}
}

// Scope accepts exactly one populated side that matches its type and rejects
// missing, crossed, or unknown scopes.
func TestScope(t *testing.T) {
	cases := []struct {
		name  string
		scope planseps.ScopeRequest
		want  error
	}{
		{"apps ok", planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{applicationID}}, nil},
		{"namespaces ok", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"ns1"}}, nil},
		{"unknown type", planseps.ScopeRequest{Type: "bogus"}, validation.ErrInvalidScope},
		{"apps empty", planseps.ScopeRequest{Type: plans.ScopeTypeApplications}, validation.ErrScopeUnion},
		{"apps crossed", planseps.ScopeRequest{
			Type: plans.ScopeTypeApplications, ApplicationRefs: []string{"a"}, Namespaces: []string{crossedNamespace},
		}, validation.ErrScopeUnion},
		{"namespaces empty", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces}, validation.ErrScopeUnion},
		{"namespaces crossed", planseps.ScopeRequest{
			Type: plans.ScopeTypeNamespaces, Namespaces: []string{crossedNamespace}, ApplicationRefs: []string{"a"},
		}, validation.ErrScopeUnion},
		{"namespaces with exclusion resources", planseps.ScopeRequest{
			Type: plans.ScopeTypeNamespaces, Namespaces: []string{"ns1"}, Exclusions: resourceExclusions(),
		}, validation.ErrExclusionResourcesScope},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validation.Scope(c.scope); !errors.Is(got, c.want) {
				t.Fatalf("Scope(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

// Policies requires at least one item, rejects unknown template ids and a template listed
// twice, and accepts a known template whose scope is supported.
func TestPolicies(t *testing.T) {
	if err := validation.Policies(nil, plans.ScopeTypeApplications); !errors.Is(err, validation.ErrPoliciesRequired) {
		t.Fatalf("empty policies = %v, want ErrPoliciesRequired", err)
	}
	if err := validation.Policies([]planseps.PolicyRequest{{TemplateID: "no-such-template"}}, plans.ScopeTypeApplications); err == nil {
		t.Fatal("unknown template should be rejected")
	}
	twice := []planseps.PolicyRequest{
		{TemplateID: tplBlockImageTags, Params: map[string]any{paramTags: []any{tagLatest}}},
		{TemplateID: tplBlockImageTags, Params: map[string]any{paramTags: []any{"dev"}}},
	}
	if err := validation.Policies(twice, plans.ScopeTypeApplications); !validation.IsValidation(err) {
		t.Fatalf("template listed twice = %v, want a validation error", err)
	}
	ok := validation.Policies([]planseps.PolicyRequest{{TemplateID: tplBlockCreate}}, plans.ScopeTypeApplications)
	testutil.Equal(t, "known template", ok, nil)
}

// TimeRange names what is wrong: absent, unparsable, or inverted; TimeRangeOpen also rejects a
// window that has already ended.
func TestTimeRange(t *testing.T) {
	cases := []struct {
		name string
		tr   *planseps.TimeRangeRequest
		want error
	}{
		{"nil", nil, validation.ErrTimeRangeRequired},
		{"unparseable", &planseps.TimeRangeRequest{StartAt: "tomorrow", EndAt: "y"}, validation.ErrTimeRangeFormat},
		{"inverted", &planseps.TimeRangeRequest{StartAt: validEndAt, EndAt: validStartAt}, validation.ErrInvalidTimeRange},
		{validCase, &planseps.TimeRangeRequest{StartAt: validStartAt, EndAt: validEndAt}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := validation.TimeRange(c.tr); !errors.Is(err, c.want) {
				t.Fatalf("TimeRange(%s) = %v, want %v", c.name, err, c.want)
			}
		})
	}
	window := &planseps.TimeRangeRequest{StartAt: validStartAt, EndAt: validEndAt}
	testutil.Equal(t, "open window", validation.TimeRangeOpen(window, beforeWindow), nil)
	if err := validation.TimeRangeOpen(window, afterWindow); !errors.Is(err, validation.ErrTimeRangeElapsed) {
		t.Fatalf("elapsed window = %v, want ErrTimeRangeElapsed", err)
	}
}

// Repeated targets and identical policies are dropped silently; the CR would otherwise list
// the same rendered policy twice.
func TestNormalize(t *testing.T) {
	req := &planseps.PrepareProtectionPlanRequest{
		Scope: planseps.ScopeRequest{
			Type:            plans.ScopeTypeNamespaces,
			Namespaces:      []string{"b", appNamespace, "b"},
			ApplicationRefs: []string{applicationID, applicationID},
		},
		Policies: []planseps.PolicyRequest{
			{TemplateID: tplBlockCreate},
			{TemplateID: tplBlockImageTags, Params: map[string]any{paramTags: []any{tagLatest}}},
			{TemplateID: tplBlockCreate},
			{TemplateID: tplBlockImageTags, Params: map[string]any{paramTags: []any{tagLatest}}},
		},
	}
	validation.Normalize(req)
	if !slices.Equal(req.Scope.Namespaces, []string{appNamespace, "b"}) {
		t.Fatalf("namespaces = %v", req.Scope.Namespaces)
	}
	if !slices.Equal(req.Scope.ApplicationRefs, []string{applicationID}) {
		t.Fatalf("applicationRefs = %v", req.Scope.ApplicationRefs)
	}
	testutil.Equal(t, "policies", len(req.Policies), constants.TwoValue)
}

// The release namespace is skipped by the policy engine, so a plan there protects nothing.
func TestPlatformNamespaces(t *testing.T) {
	testutil.Equal(t, "other", validation.PlatformNamespaces([]string{appNamespace}, ownNamespace), nil)
	testutil.Equal(t, "unknown own", validation.PlatformNamespaces([]string{appNamespace}, constants.EmptyString), nil)
	err := validation.PlatformNamespaces([]string{appNamespace, ownNamespace}, ownNamespace)
	testutil.Equal(t, "own rejected", validation.IsValidation(err), true)
}

// PrepareRequest chains scope, policy and time-range validation, surfacing the
// first failure and passing a fully valid request.
func TestPrepareRequest(t *testing.T) {
	valid := &planseps.PrepareProtectionPlanRequest{
		Name:     "p",
		Severity: plans.SeverityHigh,
		Mode:     plans.ModeEnforce,
		Scope:    planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{applicationID}},
		Policies: []planseps.PolicyRequest{{TemplateID: "block-create"}},
		TimeMode: plans.TimeModeTimeRange,
		TimeRange: &planseps.TimeRangeRequest{
			StartAt: validStartAt, EndAt: validEndAt,
		},
	}
	testutil.Equal(t, validCase, validation.PrepareRequest(valid, beforeWindow), nil)

	if err := validation.PrepareRequest(valid, afterWindow); !errors.Is(err, validation.ErrTimeRangeElapsed) {
		t.Fatalf("elapsed window = %v, want ErrTimeRangeElapsed", err)
	}

	badScope := &planseps.PrepareProtectionPlanRequest{Scope: planseps.ScopeRequest{Type: "bogus"}}
	if validation.PrepareRequest(badScope, beforeWindow) == nil {
		t.Fatal("bad scope should fail PrepareRequest")
	}

	badPolicies := &planseps.PrepareProtectionPlanRequest{
		Scope: planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{applicationID}},
	}
	if validation.PrepareRequest(badPolicies, beforeWindow) == nil {
		t.Fatal("missing policies should fail PrepareRequest")
	}

	badExclusions := *valid
	badExclusions.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{subresourceKind}}
	if err := validation.PrepareRequest(&badExclusions, beforeWindow); !errors.Is(err, validation.ErrExclusionKindInvalid) {
		t.Fatalf("subresource exclusion kind = %v, want ErrExclusionKindInvalid", err)
	}
}

// Exclusions bounds kinds and resources like the CRD and allows resources only on applications scope.
func TestExclusions(t *testing.T) {
	apps := func(e *plans.ProtectionPlanScopeExclusions) planseps.ScopeRequest {
		return planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{applicationID}, Exclusions: e}
	}
	namespaces := func(e *plans.ProtectionPlanScopeExclusions) planseps.ScopeRequest {
		return planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{appNamespace}, Exclusions: e}
	}
	kinds := func(k ...string) *plans.ProtectionPlanScopeExclusions {
		return &plans.ProtectionPlanScopeExclusions{Kinds: k}
	}
	noNamespace := excludedResource()
	noNamespace.Namespace = constants.EmptyString
	cases := []struct {
		name  string
		scope planseps.ScopeRequest
		ok    bool
		want  error
	}{
		{"nil", namespaces(nil), true, nil},
		{"kinds on namespaces", namespaces(kinds(excludedKind)), true, nil},
		{"resources on applications", apps(resourceExclusions()), true, nil},
		{"resources on namespaces", namespaces(resourceExclusions()), false, validation.ErrExclusionResourcesScope},
		{"too many kinds", apps(kinds(slices.Repeat([]string{excludedKind}, plans.ExclusionKindsMax+1)...)), false, nil},
		{"subresource kind", apps(kinds(subresourceKind)), false, validation.ErrExclusionKindInvalid},
		{"empty kind", apps(kinds(" ")), false, validation.ErrExclusionKindInvalid},
		{"resource without namespace", apps(&plans.ProtectionPlanScopeExclusions{
			Resources: []plans.ProtectionPlanExcludedResource{noNamespace},
		}), false, validation.ErrExclusionResourceInvalid},
		{"resource with subresource kind", apps(&plans.ProtectionPlanScopeExclusions{
			Resources: []plans.ProtectionPlanExcludedResource{{Kind: subresourceKind, Name: "web", Namespace: "shop"}},
		}), false, validation.ErrExclusionResourceInvalid},
		{"resource kind too long", apps(&plans.ProtectionPlanScopeExclusions{
			Resources: []plans.ProtectionPlanExcludedResource{
				{Kind: strings.Repeat("k", plans.ExclusionKindMaxLength+1), Name: "web", Namespace: "shop"},
			},
		}), false, validation.ErrExclusionResourceInvalid},
		{"resource namespace too long", apps(&plans.ProtectionPlanScopeExclusions{
			Resources: []plans.ProtectionPlanExcludedResource{
				{Kind: excludedKind, Name: "web", Namespace: strings.Repeat("n", plans.ExclusionNamespaceMaxLength+1)},
			},
		}), false, validation.ErrExclusionResourceInvalid},
		{"too many resources", apps(&plans.ProtectionPlanScopeExclusions{
			Resources: slices.Repeat([]plans.ProtectionPlanExcludedResource{excludedResource()}, plans.ExclusionResourcesMax+1),
		}), false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.Exclusions(c.scope)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
			if c.want != nil && !errors.Is(err, c.want) {
				t.Fatalf("Exclusions(%s) = %v, want %v", c.name, err, c.want)
			}
		})
	}
}

// Fields enforces every limit the CRD schema enforces, so a rejected plan never
// reaches the cluster.
func TestFields(t *testing.T) {
	longName := strings.Repeat(crossedNamespace, validation.NameMaxLength+1)
	longDescription := strings.Repeat("d", validation.DescriptionMaxLength+1)
	longTaxonomyID := strings.Repeat("t", validation.TaxonomyIDMaxLength+1)
	tooManyTags := make([]string, validation.TagRefsMax+1)
	for i := range tooManyTags {
		tooManyTags[i] = fmt.Sprintf("cat-00003-0001-%04d", i)
	}
	cases := []struct {
		name  string
		mutil func(*planseps.PrepareProtectionPlanRequest)
		ok    bool
	}{
		{validCase, func(*planseps.PrepareProtectionPlanRequest) {}, true},
		{"blank name", func(r *planseps.PrepareProtectionPlanRequest) { r.Name = "   " }, false},
		{"long name", func(r *planseps.PrepareProtectionPlanRequest) { r.Name = longName }, false},
		{"long description", func(r *planseps.PrepareProtectionPlanRequest) { r.Description = &longDescription }, false},
		{"bad severity", func(r *planseps.PrepareProtectionPlanRequest) { r.Severity = "extreme" }, false},
		{"bad mode", func(r *planseps.PrepareProtectionPlanRequest) { r.Mode = "warn" }, false},
		{"bad time mode", func(r *planseps.PrepareProtectionPlanRequest) { r.TimeMode = "forever" }, false},
		{"priority too low", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Priority = validation.PriorityMin - constants.DefaultAddValue
		}, false},
		{"priority too high", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Priority = validation.PriorityMax + constants.DefaultAddValue
		}, false},
		{"nil taxonomy ok", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentRef = nil; r.TagRefs = nil }, true},
		{"environmentRef too long", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentRef = &longTaxonomyID }, false},
		{"empty environmentRef ok", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentRef = strptr("") }, true},
		{"tag id too long", func(r *planseps.PrepareProtectionPlanRequest) { r.TagRefs = []string{longTaxonomyID} }, false},
		{"too many tag ids", func(r *planseps.PrepareProtectionPlanRequest) { r.TagRefs = tooManyTags }, false},
		{"duplicate tag ids", func(r *planseps.PrepareProtectionPlanRequest) { r.TagRefs = []string{taxonomyTagID, taxonomyTagID} }, false},
		{"valid taxonomy", func(r *planseps.PrepareProtectionPlanRequest) {
			r.EnvironmentRef = strptr(taxonomyEnvironmentRef)
			r.TagRefs = []string{taxonomyTagID, taxonomyTagB}
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &planseps.PrepareProtectionPlanRequest{
				Name:     "plan",
				Severity: plans.SeverityLow,
				Mode:     plans.ModeAudit,
				TimeMode: plans.TimeModePermanent,
			}
			c.mutil(req)
			err := validation.Fields(req)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
		})
	}
}

// ExcludedNamespaces rejects a scope that names any namespace discovery ignores.
func TestExcludedNamespaces(t *testing.T) {
	cases := []struct {
		name       string
		namespaces []string
		excluded   []string
		ok         bool
	}{
		{"none excluded", []string{appNamespace}, []string{kubeSystemNamespace}, true},
		{"no exclusions configured", []string{appNamespace}, nil, true},
		{"one excluded", []string{appNamespace, kubeSystemNamespace}, []string{kubeSystemNamespace}, false},
		{"all excluded", []string{kubeSystemNamespace}, []string{kubeSystemNamespace, "kube-public"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.ExcludedNamespaces(c.namespaces, c.excluded)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
		})
	}
}

// MissingNamespaces rejects a scope naming a namespace the cluster does not have.
func TestMissingNamespaces(t *testing.T) {
	cases := []struct {
		name       string
		namespaces []string
		existing   []string
		ok         bool
	}{
		{"all present", []string{appNamespace}, []string{appNamespace, "db"}, true},
		{"one missing", []string{appNamespace, "ghost"}, []string{appNamespace}, false},
		{"empty cluster", []string{appNamespace}, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.MissingNamespaces(c.namespaces, c.existing)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
		})
	}
}

// UniqueName compares names case-insensitively after trimming and ignores the plan being edited.
// A taken name is a conflict (409), not a malformed request.
func TestUniqueName(t *testing.T) {
	existing := []plans.ProtectionPlan{{ID: prodGuardID, Name: prodGuardName}, {ID: "pp-2", Name: "Staging"}}
	cases := []struct {
		name      string
		candidate string
		excludeID string
		ok        bool
	}{
		{"free", "New Plan", constants.EmptyString, true},
		{"exact clash", prodGuardName, constants.EmptyString, false},
		{"case clash", "prod guard", constants.EmptyString, false},
		{"padded clash", "  Prod Guard  ", constants.EmptyString, false},
		{"same plan renamed to itself", prodGuardName, prodGuardID, true},
		{"other plan name", "Staging", prodGuardID, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.UniqueName(existing, c.candidate, c.excludeID)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "conflict", validation.IsConflict(err), true)
				testutil.Equal(t, "not validation", validation.IsValidation(err), false)
			}
		})
	}
}

// Fields accepts only the two execution modes the CRD enum allows.
func TestFieldsRejectsUnknownApprovalMode(t *testing.T) {
	cases := []struct {
		name string
		mode *string
		ok   bool
	}{
		{"absent", nil, true},
		{"automatic", strptr(plans.ApprovalModeAutomatic), true},
		{"required", strptr(plans.ApprovalModeRequired), true},
		{"manual", strptr("manual"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &planseps.PrepareProtectionPlanRequest{
				Name:         "plan",
				Severity:     plans.SeverityLow,
				Mode:         plans.ModeAudit,
				TimeMode:     plans.TimeModePermanent,
				ApprovalMode: c.mode,
			}
			err := validation.Fields(req)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
		})
	}
}
