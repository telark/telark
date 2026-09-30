package application

type ApplicationMetrics struct {
	Derived   DerivedMetrics  `json:"derived"`
	Workloads []WorkloadUsage `json:"workloads"`
}

type DerivedMetrics struct {
	TotalChanges          int            `json:"totalChanges"`
	ChangesByClass        map[string]int `json:"changesByClass"`
	ChangesBySeverity     map[string]int `json:"changesBySeverity"`
	TotalIncidents        int            `json:"totalIncidents"`
	TotalRecoveries       int            `json:"totalRecoveries"`
	SnapshotCount         int            `json:"snapshotCount"`
	FirstChangeDetectedAt *string        `json:"firstChangeDetectedAt,omitempty"`
	LastChangeDetectedAt  *string        `json:"lastChangeDetectedAt,omitempty"`
	ChangeVelocityPerDay  float64        `json:"changeVelocityPerDay"`
	UniqueFingerprints    int            `json:"uniqueFingerprints"`
}

type WorkloadUsage struct {
	ResourceName string          `json:"resourceName"`
	ResourceKind string          `json:"resourceKind"`
	Namespace    string          `json:"namespace"`
	Baseline     MetricsBaseline `json:"baseline"`
	Usage        Usage           `json:"usage"`
}

type MetricsBaseline struct {
	// SHA256[:8] of: image + requestsCPU + requestsMemory + limitsCPU + limitsMemory + replicas
	Fingerprint string         `json:"fingerprint"`
	Replicas    int32          `json:"replicas"`
	Requests    ResourceValues `json:"requests"`
	Limits      ResourceValues `json:"limits"`
}

type ResourceValues struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

func NewApplicationMetrics() ApplicationMetrics {
	return ApplicationMetrics{
		Derived:   DerivedMetrics{ChangesByClass: map[string]int{}, ChangesBySeverity: map[string]int{}},
		Workloads: []WorkloadUsage{},
	}
}
