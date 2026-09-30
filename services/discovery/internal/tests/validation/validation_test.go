package validation

import (
	"testing"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

var (
	apps = string(plans.ScopeTypeApplications)
	nss  = string(plans.ScopeTypeNamespaces)
)

// A plan scope must be exactly one of applications (with ids, no namespaces) or
// namespaces (with namespaces, no ids); anything else is rejected before the
// plan is stored.
func TestScope(t *testing.T) {
	cases := []struct {
		name    string
		scope   planseps.ScopeRequest
		wantErr bool
	}{
		{"apps valid", planseps.ScopeRequest{Type: apps, ApplicationRefs: []string{"a1"}}, false},
		{"apps missing ids", planseps.ScopeRequest{Type: apps}, true},
		{"apps with namespaces", planseps.ScopeRequest{Type: apps, ApplicationRefs: []string{"a1"}, Namespaces: []string{"n"}}, true},
		{"namespaces valid", planseps.ScopeRequest{Type: nss, Namespaces: []string{"n"}}, false},
		{"namespaces missing", planseps.ScopeRequest{Type: nss}, true},
		{"unknown type", planseps.ScopeRequest{Type: "bogus"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "err", validation.Scope(c.scope) != nil, c.wantErr)
		})
	}
}

// A plan needs at least one policy, and every policy must name a known template.
func TestPolicies(t *testing.T) {
	if validation.Policies(nil, apps) == nil {
		t.Fatal("empty policy list accepted")
	}
	unknown := []planseps.PolicyRequest{{TemplateID: "no-such-template"}}
	if validation.Policies(unknown, apps) == nil {
		t.Fatal("unknown template accepted")
	}
}

// A time-ranged plan must end after it starts.
func TestTimeRange(t *testing.T) {
	if err := validation.TimeRange(&planseps.TimeRangeRequest{StartAt: "2030-01-01T00:00:00Z", EndAt: "2030-01-02T00:00:00Z"}); err != nil {
		t.Fatalf("valid range rejected: %v", err)
	}
	if validation.TimeRange(&planseps.TimeRangeRequest{StartAt: "2030-01-02T00:00:00Z", EndAt: "2030-01-01T00:00:00Z"}) == nil {
		t.Fatal("end-before-start accepted")
	}
}
