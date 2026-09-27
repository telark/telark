package constants

// Fan-out caps, all held well under the K8s client QPS.
const (
	ViolationListConcurrency   = 8
	PolicyOpConcurrency        = 8
	HealthReconcileConcurrency = 4
)
