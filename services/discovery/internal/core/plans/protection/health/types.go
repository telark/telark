package health

import (
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
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
	fieldSpec             = "spec"
	fieldRules            = "rules"
	stageRepair           = "repair"
	stagePhaseRecheck     = "phase-recheck"
	stageFirstCheck       = "first-check"
	fmtRenderUnavailable  = "current render unavailable: %w"
	logRepairedFmt        = "protection-plan repaired plan=%s redeployed=%v repatched=%v removed=%v added=%v"
	logSnapshotFailedFmt  = "protection-plan health snapshot failed err=%v"
	logResolveFailedFmt   = "protection-plan health application resolve failed err=%v"
	logOrphansSweptFmt    = "protection-plan orphan sweep removed policies=%v"
	logOrphanSweepFailFmt = "protection-plan orphan sweep failed err=%v"
	// Outlives a deploy budget: Prepare and approve deploy before the plan is created or active,
	// so a younger policy may belong to a write still in flight.
	OrphanGracePeriod = 4 * constants.ProtectionPlanDeployTimeout
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
	// Present but rendered by an older renderer or plan revision; redeployed like Missing.
	Stale []string
	// Rendered now but never listed by the plan (a namespace an older renderer skipped); deployed
	// and added to the plan's rendered set, which Rendered then carries into the health patch.
	Added    []string
	Rendered []string
	// The render the check compared against, so the repair redeploys exactly that.
	current   map[string]kyvernov1.Policy
	renderErr error
}

type repairOutcome struct {
	redeployed []string
	repatched  []string
	removed    []string
	added      []string
}

type policySnapshot struct {
	created       time.Time
	namespace     string
	ready         bool
	failureAction string
	renderHash    string
	rules         []kyvernov1.Rule
}

type healthFlags struct {
	notReady bool
	drifted  bool
}
