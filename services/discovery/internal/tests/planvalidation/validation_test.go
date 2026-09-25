package planvalidation

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	taxonomyEnvironmentID = "cat-00002-0001-0001"
	taxonomyTagID         = "cat-00003-0001-0001"
	taxonomyTagB          = "cat-00003-0001-0002"
	validStartAt          = "2026-01-01T00:00:00Z"
	applicationID         = "a1"
	crossedNamespace      = "n"
	validCase             = "valid"
	appNamespace          = "app"
	prodGuardName         = "Prod Guard"
	prodGuardID           = "pp-1"
	validEndAt            = "2026-01-02T00:00:00Z"
	kubeSystemNamespace   = "kube-system"
	excludedKind          = "ConfigMap"
	excludedName          = "wa1"
	subresourceKind       = "Deployment/scale"
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
		{"apps ok", planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{applicationID}}, nil},
		{"namespaces ok", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"ns1"}}, nil},
		{"unknown type", planseps.ScopeRequest{Type: "bogus"}, validation.ErrInvalidScope},
		{"apps empty", planseps.ScopeRequest{Type: plans.ScopeTypeApplications}, validation.ErrScopeUnion},
		{"apps crossed", planseps.ScopeRequest{
			Type: plans.ScopeTypeApplications, ApplicationIDs: []string{"a"}, Namespaces: []string{crossedNamespace},
		}, validation.ErrScopeUnion},
		{"namespaces empty", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces}, validation.ErrScopeUnion},
		{"namespaces crossed", planseps.ScopeRequest{
			Type: plans.ScopeTypeNamespaces, Namespaces: []string{crossedNamespace}, ApplicationIDs: []string{"a"},
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

// Policies requires at least one item, rejects unknown template ids, and accepts
// a known template whose scope is supported.
func TestPolicies(t *testing.T) {
	if err := validation.Policies(nil, plans.ScopeTypeApplications); !errors.Is(err, validation.ErrPoliciesRequired) {
		t.Fatalf("empty policies = %v, want ErrPoliciesRequired", err)
	}
	if err := validation.Policies([]planseps.PolicyRequest{{TemplateID: "no-such-template"}}, plans.ScopeTypeApplications); err == nil {
		t.Fatal("unknown template should be rejected")
	}
	ok := validation.Policies([]planseps.PolicyRequest{{TemplateID: "block-create"}}, plans.ScopeTypeApplications)
	testutil.Equal(t, "known template", ok, nil)
}

// TimeRange requires a present, parseable window whose end is after its start.
func TestTimeRange(t *testing.T) {
	cases := []struct {
		name string
		tr   *planseps.TimeRangeRequest
		ok   bool
	}{
		{"nil", nil, false},
		{"unparseable", &planseps.TimeRangeRequest{StartAt: "x", EndAt: "y"}, false},
		{"inverted", &planseps.TimeRangeRequest{StartAt: validEndAt, EndAt: validStartAt}, false},
		{validCase, &planseps.TimeRangeRequest{StartAt: validStartAt, EndAt: validEndAt}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.TimeRange(c.tr)
			testutil.Equal(t, "ok", err == nil, c.ok)
		})
	}
}

// PrepareRequest chains scope, policy and time-range validation, surfacing the
// first failure and passing a fully valid request.
func TestPrepareRequest(t *testing.T) {
	valid := &planseps.PrepareProtectionPlanRequest{
		Name:     "p",
		Severity: plans.SeverityHigh,
		Mode:     plans.ModeEnforce,
		Scope:    planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{applicationID}},
		Policies: []planseps.PolicyRequest{{TemplateID: "block-create"}},
		TimeMode: plans.TimeModeTimeRange,
		TimeRange: &planseps.TimeRangeRequest{
			StartAt: validStartAt, EndAt: validEndAt,
		},
	}
	testutil.Equal(t, validCase, validation.PrepareRequest(valid), nil)

	badScope := &planseps.PrepareProtectionPlanRequest{Scope: planseps.ScopeRequest{Type: "bogus"}}
	if validation.PrepareRequest(badScope) == nil {
		t.Fatal("bad scope should fail PrepareRequest")
	}

	badPolicies := &planseps.PrepareProtectionPlanRequest{
		Scope: planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{applicationID}},
	}
	if validation.PrepareRequest(badPolicies) == nil {
		t.Fatal("missing policies should fail PrepareRequest")
	}

	badExclusions := *valid
	badExclusions.Scope.Exclusions = &plans.ProtectionPlanScopeExclusions{Kinds: []string{subresourceKind}}
	if err := validation.PrepareRequest(&badExclusions); !errors.Is(err, validation.ErrExclusionKindInvalid) {
		t.Fatalf("subresource exclusion kind = %v, want ErrExclusionKindInvalid", err)
	}
}

// Exclusions bounds kinds and resources like the CRD and allows resources only on applications scope.
func TestExclusions(t *testing.T) {
	apps := func(e *plans.ProtectionPlanScopeExclusions) planseps.ScopeRequest {
		return planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{applicationID}, Exclusions: e}
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
	tooManyTags := make([]string, validation.TagIDsMax+1)
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
		{"nil taxonomy ok", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentID = nil; r.TagIDs = nil }, true},
		{"environmentID too long", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentID = &longTaxonomyID }, false},
		{"empty environmentID ok", func(r *planseps.PrepareProtectionPlanRequest) { r.EnvironmentID = strptr("") }, true},
		{"tag id too long", func(r *planseps.PrepareProtectionPlanRequest) { r.TagIDs = []string{longTaxonomyID} }, false},
		{"too many tag ids", func(r *planseps.PrepareProtectionPlanRequest) { r.TagIDs = tooManyTags }, false},
		{"duplicate tag ids", func(r *planseps.PrepareProtectionPlanRequest) { r.TagIDs = []string{taxonomyTagID, taxonomyTagID} }, false},
		{"valid taxonomy", func(r *planseps.PrepareProtectionPlanRequest) {
			r.EnvironmentID = strptr(taxonomyEnvironmentID)
			r.TagIDs = []string{taxonomyTagID, taxonomyTagB}
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
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
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
