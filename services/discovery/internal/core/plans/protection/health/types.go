package health

import (
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	planseps "github.com/telark/rest/endpoints/plans"
)

const (
	CheckTimeoutSeconds = 15
	// Kyverno stamps Ready from its own webhook reconcile, so a check fired the instant a
	// plan goes active always reads "not ready". Raise this if that pass gets slower.
	firstCheckDelaySeconds = 5
	FirstCheckBudget       = time.Duration(firstCheckDelaySeconds+CheckTimeoutSeconds) * time.Second

	kyvernoConditionReady = "Ready"
	kyvernoStatusTrue     = "True"
	kyvernoActionEnforce  = "Enforce"
	kyvernoActionAudit    = "Audit"
	stageRepair           = "repair"
	stagePhaseRecheck     = "phase-recheck"
	stageFirstCheck       = "first-check"
	errMissingAppsFmt     = "applications not found: %v"
	logRepairedFmt        = "protection-plan repaired plan=%s redeployed=%v repatched=%v removed=%v"
	logSnapshotFailedFmt  = "protection-plan health snapshot failed err=%v"
)

var firstCheckSlots = make(chan struct{}, constants.HealthReconcileConcurrency)

// Narrow view of the exporter client so the repair guard can be exercised with a fake.
type PlanStore interface {
	Get(planID string) (*plans.ProtectionPlan, error)
	PatchOrError(userID, planID string, req planseps.PatchProtectionPlanRequest) error
}

type Result struct {
	Health     string
	Detail     []plans.ProtectionPlanHealthDetail
	Policies   []planseps.ProtectionPlanPolicyStatus
	Missing    []string
	Mismatched []string
	Unexpected []string
}

type repairOutcome struct {
	redeployed []string
	repatched  []string
	removed    []string
}

type policySnapshot struct {
	namespace     string
	ready         bool
	failureAction string
}

type healthFlags struct {
	notReady bool
	drifted  bool
}
