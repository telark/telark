package planupdate

import (
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/update"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const stagingEnv = "cat-00002-0001-0002"

// Creating in staging (automatic) and then editing the environment to Production would label
// the plan with a gate it never passed.
func TestEnvironmentRaisesApproval(t *testing.T) {
	prod := dataconstants.CategoryIDEnvProduction
	stagingProd := prod
	cases := []struct {
		name   string
		stored plans.ProtectionPlan
		env    *string
		want   bool
	}{
		{"automatic staging moved to production", plans.ProtectionPlan{EnvironmentRef: stagingEnv}, &stagingProd, true},
		{
			"required plan moved to production",
			plans.ProtectionPlan{EnvironmentRef: stagingEnv, ApprovalMode: plans.ApprovalModeRequired},
			&stagingProd, false,
		},
		{"environment unchanged", plans.ProtectionPlan{EnvironmentRef: prod}, &stagingProd, false},
		{"environment not sent", plans.ProtectionPlan{EnvironmentRef: stagingEnv}, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &planseps.PrepareProtectionPlanRequest{EnvironmentRef: c.env}
			testutil.Equal(t, "raises", update.EnvironmentRaisesApproval(&c.stored, req), c.want)
		})
	}
}
