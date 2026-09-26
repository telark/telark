package planupdate

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/core/plans/protection/update"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
)

const renamedPlan = "guard-renamed"

// The regression: a rename triggered the full render, but the render target kept the stored
// name, so every live policy was redeployed with the old name in its messages and annotation.
func TestRenderTargetCarriesTheNewName(t *testing.T) {
	plan := &plans.ProtectionPlan{
		Name:  "guard",
		Mode:  plans.ModeAudit,
		Scope: plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{otherNS}},
	}
	req := &planseps.PrepareProtectionPlanRequest{
		Name:  renamedPlan,
		Mode:  plans.ModeEnforce,
		Scope: planseps.ScopeRequest{Type: plans.ScopeTypeNamespaces, Namespaces: []string{otherNS}},
	}

	target := update.RenderTarget(plan, req, nil)

	testutil.Equal(t, "name", target.Name, renamedPlan)
	testutil.Equal(t, "mode", target.Mode, plans.ModeEnforce)
	testutil.Equal(t, "stored plan untouched", plan.Name, "guard")
}
