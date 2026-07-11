package constants

const (
	// ViolationListConcurrency caps concurrent dynamic.List(PolicyReport) calls
	// when fanning out per-namespace violation lookups.
	ViolationListConcurrency = 8

	// PolicyOpConcurrency caps concurrent Kyverno Policy Delete/Patch calls when
	// the applier processes a list result. Bounded well under K8s QPS.
	PolicyOpConcurrency = 8

	// HealthReconcileConcurrency caps concurrent per-plan health Compute calls
	// during the protection-plan controller tick.
	HealthReconcileConcurrency = 4
)
