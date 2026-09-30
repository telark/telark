package planhealth

import (
	"context"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/health"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	activePlanID   = "pp-act-1234-0001"
	canceledPlanID = "pp-can-1234-0002"
	deletedPlanID  = "pp-del-1234-0003"
	inFlightPlanID = "pp-new-1234-0004"
	activePolicy   = "pol-active"
	canceledPolicy = "pol-canceled"
	deletedPolicy  = "pol-deleted"
	inFlightPolicy = "pol-inflight"
	freshAge       = 10 * time.Second
)

var sweepNow = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

func agedPolicy(name, planID string, age time.Duration) runtime.Object {
	obj := kyvernoPolicy(name, planID, modeEnforce, true)
	obj.SetCreationTimestamp(metav1.NewTime(sweepNow.Add(-age)))
	return obj
}

// Policies whose plan was cleared mid-deploy, or whose cleanup failed after the phase patch,
// enforce with no plan left to cancel: the leader tick removes them, but never a policy young
// enough to belong to a create or approve still in flight.
func TestReconcileSweepsOrphanPolicies(t *testing.T) {
	old := constants.TwoValue * health.OrphanGracePeriod
	dyn := fakeDyn(
		agedPolicy(activePolicy, activePlanID, old),
		agedPolicy(canceledPolicy, canceledPlanID, old),
		agedPolicy(deletedPolicy, deletedPlanID, old),
		agedPolicy(inFlightPolicy, inFlightPlanID, freshAge),
	)
	planList := []plans.ProtectionPlan{
		{ID: activePlanID, Phase: plans.PhaseActive, Mode: plans.ModeEnforce, RenderedPolicies: []string{activePolicy}},
		{ID: canceledPlanID, Phase: plans.PhaseCanceled, Mode: plans.ModeEnforce},
	}
	deps := repairDeps(dyn, &recordingLogger{})
	deps.Clock = func() time.Time { return sweepNow }

	health.ReconcileForActive(context.Background(), deps, planList)

	cases := []struct {
		name string
		kept bool
	}{
		{activePolicy, true},
		{canceledPolicy, false},
		{deletedPolicy, false},
		{inFlightPolicy, true},
	}
	for _, c := range cases {
		testutil.Equal(t, c.name+" kept", getPolicy(t, dyn, c.name) != nil, c.kept)
	}
}

type fullPlanStore struct {
	plan    *plans.ProtectionPlan
	patches int
}

func (f *fullPlanStore) Get(string) (*plans.ProtectionPlan, error) {
	clone := *f.plan
	return &clone, nil
}

func (f *fullPlanStore) PatchOrError(string, string, planseps.PatchProtectionPlanRequest) error {
	f.patches++
	return nil
}

// The status route is Read on protection plans: a viewer poll must neither redeploy a policy an
// operator removed as break-glass nor write the plan.
func TestReadIsComputeOnly(t *testing.T) {
	plan, policyName := renderedPlan(t, plans.PhaseActive)
	dyn := withApplyCreate(fakeDyn())
	writes := constants.DefaultInitValue
	dyn.PrependReactor("*", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetVerb() != "list" && action.GetVerb() != "get" {
			writes++
		}
		return false, nil, nil
	})
	store := &fullPlanStore{plan: plan}
	deps := repairDeps(dyn, &recordingLogger{})
	deps.Exporter = store

	_, result, err := health.Read(context.Background(), deps, plan.ID)
	testutil.Equal(t, labelErr, err, nil)
	testutil.Equal(t, "missing reported", len(result.Missing) > 0, true)
	testutil.Equal(t, "policy redeployed", getPolicy(t, dyn, policyName) != nil, false)
	testutil.Equal(t, "cluster writes", writes, constants.DefaultInitValue)
	testutil.Equal(t, "plan patches", store.patches, constants.DefaultInitValue)
}
