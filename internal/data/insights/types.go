package insights

// Signal is the per-application input discovery derives from an Application CRD
// and hands to the enrichment service.
type Signal struct {
	Name          string   `json:"name"`
	Namespace     string   `json:"namespace"`
	Images        []string `json:"images"`
	Ports         []int    `json:"ports"`
	EnvVarKeys    []string `json:"envVarKeys"`
	ResourceKinds []string `json:"resourceKinds"`
	HasIngress    bool     `json:"hasIngress"`
	HasPVC        bool     `json:"hasPVC"`

	// HA / health
	Replicas      int    `json:"replicas"`
	ReadyReplicas int    `json:"readyReplicas"`
	HealthStatus  string `json:"healthStatus"`

	// posture
	WorkloadKinds    []string `json:"workloadKinds"`
	HasService       bool     `json:"hasService"`
	HasHPA           bool     `json:"hasHPA"`
	HasNetworkPolicy bool     `json:"hasNetworkPolicy"`

	// config source
	SecretRefs    []string `json:"secretRefs"`
	ConfigMapRefs []string `json:"configMapRefs"`

	// managed
	ManagedBy string `json:"managedBy"`
	Chart     string `json:"chart"`

	// stability
	ChangeVelocityPerDay float64 `json:"changeVelocityPerDay"`
	Incidents            int     `json:"incidents"`
	Recoveries           int     `json:"recoveries"`

	// per-workload compact usage
	Workloads []WorkloadSignal `json:"workloads"`
}

type WorkloadSignal struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Replicas  int    `json:"replicas"`
	QoS       string `json:"qos"`
	CPU       string `json:"cpu"`
	Memory    string `json:"memory"`
	LimitsSet bool   `json:"limitsSet"`
}

type DispatchRequest struct {
	Items []Signal `json:"items"`
}
