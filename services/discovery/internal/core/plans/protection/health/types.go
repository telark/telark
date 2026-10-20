package health

import (
	"time"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	CheckTimeoutSeconds = 15
	// Kyverno stamps Ready from its own webhook reconcile, so a check fired the instant a
	// plan goes active always reads "not ready". Raise this if that pass gets slower.
	firstCheckDelaySeconds = 5
	FirstCheckBudget       = time.Duration(firstCheckDelaySeconds+CheckTimeoutSeconds) * time.Second

	kyvernoActionEnforce  = "Enforce"
	kyvernoActionAudit    = "Audit"
	fieldSpec             = "spec"
	fieldStatus           = "status"
	fieldConditionStatus  = "conditionStatus"
	fieldReady            = "ready"
	fieldValidationAction = "validationActions"
	labelSelectorFmt      = "%s=%s"
	stageRepair           = "repair"
	stagePhaseRecheck     = "phase-recheck"
	stageFirstCheck       = "first-check"
	fmtRenderUnavailable  = "current render unavailable: %w"
	logRepairedFmt        = "protection-plan repaired plan=%s redeployed=%v repatched=%v removed=%v added=%v"
	logSnapshotFailedFmt  = "protection-plan health snapshot failed err=%v"
	logResolveFailedFmt   = "protection-plan health application resolve failed err=%v"
	logOrphansSweptFmt    = "protection-plan orphan sweep removed policies=%v"
	logOrphanSweepFailFmt = "protection-plan orphan sweep failed err=%v"
	logPolicyKindMissing  = "protection-plan health: the cluster does not serve namespacedvalidatingpolicies.policies.kyverno.io" +
		" (Kyverno 1.18+ CRDs); active plans read degraded and only legacy policies enforce"
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
	// Present but rendered by an older renderer or plan revision, or edited by hand; replaced whole.
	Stale []string
	// Rendered now but never listed by the plan (a namespace an older renderer skipped); deployed
	// and added to the plan's rendered set, which Rendered then carries into the health patch.
	Added    []string
	Rendered []string
	// The render the check compared against, so the repair redeploys exactly that.
	current   map[string]policiesv1.NamespacedValidatingPolicy
	renderErr error
	// The live policies it compared, which a stale one is replaced from.
	live map[string]policySnapshot
	// The cluster serves no policies.kyverno.io kind: nothing can be deployed, so nothing is repaired.
	kindMissing bool
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
	// Kyverno's own spec type, so a field the render never sets still survives the decode as drift.
	spec   *policiesv1.ValidatingPolicySpec
	object *unstructured.Unstructured
}

// The policies of one LIST pass, by plan ID, then by name.
type policySnapshots struct {
	current     map[string]map[string]policySnapshot
	kindMissing bool
}

type healthFlags struct {
	notReady bool
	drifted  bool
}
