package insights

type ClusterInsightAsResource struct {
	Name         string      `json:"name"`
	Cluster      Cluster      `json:"cluster"`
	Orchestrator Orchestrator `json:"orchestrator"`
	Metrics      Metrics      `json:"metrics"`
	Inventory    Inventory    `json:"inventory"`
}

type Cluster struct {
	UID           string `json:"uid"`
	Name          string `json:"name"`
	CloudProvider string `json:"cloudProvider"`
	Distribution  string `json:"distribution"`
	Region        string `json:"region"`
}

type Orchestrator struct {
	Version OrchestratorVersion `json:"version"`
}

type OrchestratorVersion struct {
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	Patch int    `json:"patch"`
	Full  string `json:"full"`
}

type Metrics struct {
	Available bool   `json:"available"`
	CheckedAt string `json:"checkedAt"`
}

type Inventory struct {
	Nodes      Nodes      `json:"nodes"`
	Namespaces Namespaces `json:"namespaces"`
	Workloads  Workloads  `json:"workloads"`
	Services   int        `json:"services"`
	Storage    Storage    `json:"storage"`
}

type Nodes struct {
	Total        int `json:"total"`
	ControlPlane int `json:"controlPlane"`
	Worker       int `json:"worker"`
}

type Namespaces struct {
	Total    int      `json:"total"`
	Excluded []string `json:"excluded"`
}

type Workloads struct {
	Deployments  int `json:"deployments"`
	Statefulsets int `json:"statefulsets"`
	Daemonsets   int `json:"daemonsets"`
	Jobs         int `json:"jobs"`
	Cronjobs     int `json:"cronjobs"`
	Pods         int `json:"pods"`
}

type Storage struct {
	StorageClasses int `json:"storageClasses"`
	PVCs           int `json:"pvcs"`
	PVs            int `json:"pvs"`
}
