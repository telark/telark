package policies

import (
	"context"
	"testing"

	"github.com/telark/data/plans"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/tests/testutil"
)

type noopDiffLogger struct{}

func (noopDiffLogger) Info(string)  {}
func (noopDiffLogger) Error(string) {}

func diffMsgs() protpolicies.DeployErrorMessages {
	return protpolicies.DeployErrorMessages{
		GenericDeployFailure: "deploy failed",
		InternalErrorFormat:  "internal: %s",
	}
}

// A plan that is not active is a no-op: the current rendered policies are
// returned unchanged and nothing touches the cluster.
func TestApplyClusterDiffInactivePlan(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: "pp-1", Phase: "pending", Mode: plans.ModeEnforce, RenderedPolicies: []string{"pol-a"}}
	applier := protpolicies.NewApplier(applierDyn(), nil)
	deployed, kept, err := protpolicies.ApplyClusterDiff(
		context.Background(), applier, noopDiffLogger{}, plan, nil, nil, nil, plan.Mode, diffMsgs(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "nothing deployed", len(deployed), 0)
	testutil.Equal(t, "kept rendered", len(kept), 1)
}

// An active plan with no deploy or remove combos and an unchanged mode keeps the
// rendered policies and reports no error.
func TestApplyClusterDiffNoCombos(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: "pp-1", Phase: plans.PhaseActive, Mode: plans.ModeEnforce, RenderedPolicies: []string{"pol-a", "pol-b"}}
	applier := protpolicies.NewApplier(applierDyn(policyObj("pol-a", "prod", "pp-1", "Enforce")), nil)
	deployed, kept, err := protpolicies.ApplyClusterDiff(
		context.Background(), applier, noopDiffLogger{}, plan, nil, nil, nil, plan.Mode, diffMsgs(),
	)
	testutil.Equal(t, "err", err, nil)
	testutil.Equal(t, "nothing deployed", len(deployed), 0)
	testutil.Equal(t, "kept both", len(kept), 2)
}

// RollbackPatchFailure is a no-op when nothing was deployed, and deletes the
// just-deployed policies otherwise.
func TestRollbackPatchFailure(t *testing.T) {
	plan := &plans.ProtectionPlan{ID: "pp-1"}
	dyn := applierDyn(policyObj("pol-a", "prod", "pp-1", "Enforce"))
	applier := protpolicies.NewApplier(dyn, nil)

	protpolicies.RollbackPatchFailure(context.Background(), applier, noopDiffLogger{}, plan, nil)
	testutil.Equal(t, "noop keeps policy", countPolicies(t, dyn), 1)

	protpolicies.RollbackPatchFailure(context.Background(), applier, noopDiffLogger{}, plan, []string{"pol-a"})
	testutil.Equal(t, "rolled back", countPolicies(t, dyn), 0)
}
