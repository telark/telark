package insights

type ResourceAsInsight struct {
	Cacid Cacid `json:"cacid"`
}

type Cacid struct {
	Orchestrator Orchestrator `json:"orchestrator"`
	Metrics      []Metrics    `json:"metrics"`
}

type Orchestrator struct {
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	Patch int    `json:"patch"`
	Full  string `json:"full"`
}

type Metrics struct {
	Kind        string `json:"kind"`
	Enabled     bool   `json:"enabled"`
	ServiceType string `json:"serviceType"`
	InternalIP  string `json:"internalIP"`
	Port        int    `json:"port"`
	Namespace   string `json:"namespace"`
	Host        string `json:"host"`
}
