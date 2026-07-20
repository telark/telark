package planvalidation

import (
	"errors"
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

// Scope accepts exactly one populated side that matches its type and rejects
// missing, crossed, or unknown scopes.
func TestScope(t *testing.T) {
	cases := []struct {
		name  string
		scope planseps.ScopeRequest
		want  error
	}{
		{"apps ok", planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{"a1"}}, nil},
		{"namespaces ok", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"ns1"}}, nil},
		{"unknown type", planseps.ScopeRequest{Type: "bogus"}, validation.ErrInvalidScope},
		{"apps empty", planseps.ScopeRequest{Type: plans.ScopeTypeApplications}, validation.ErrScopeUnion},
		{"apps crossed", planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{"a"}, Namespaces: []string{"n"}}, validation.ErrScopeUnion},
		{"namespaces empty", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces}, validation.ErrScopeUnion},
		{"namespaces crossed", planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"n"}, ApplicationIDs: []string{"a"}}, validation.ErrScopeUnion},
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
		{"inverted", &planseps.TimeRangeRequest{StartAt: "2026-01-02T00:00:00Z", EndAt: "2026-01-01T00:00:00Z"}, false},
		{"valid", &planseps.TimeRangeRequest{StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z"}, true},
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
		Scope:    planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{"a1"}},
		Policies: []planseps.PolicyRequest{{TemplateID: "block-create"}},
		TimeMode: plans.TimeModeTimeRange,
		TimeRange: &planseps.TimeRangeRequest{
			StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z",
		},
	}
	testutil.Equal(t, "valid", validation.PrepareRequest(valid), nil)

	badScope := &planseps.PrepareProtectionPlanRequest{Scope: planseps.ScopeRequest{Type: "bogus"}}
	if validation.PrepareRequest(badScope) == nil {
		t.Fatal("bad scope should fail PrepareRequest")
	}

	badPolicies := &planseps.PrepareProtectionPlanRequest{
		Scope: planseps.ScopeRequest{Type: plans.ScopeTypeApplications, ApplicationIDs: []string{"a1"}},
	}
	if validation.PrepareRequest(badPolicies) == nil {
		t.Fatal("missing policies should fail PrepareRequest")
	}
}
