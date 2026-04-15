package application

type Application struct {
	Name            string                `json:"name"`
	DisplayName     string                `json:"displayName"`
	Description     *string               `json:"description,omitempty"`
	Health          Health                `json:"health"`
	ResourceCount   int                   `json:"resourceCount"`
	Namespaces      Namespaces            `json:"namespaces"`
	Managed         Managed               `json:"managed"`
	CreatedAt       string                `json:"createdAt"`
	LastUpdated     string                `json:"lastUpdated"`
	ResourceSummary ResourceSummary       `json:"resourceSummary"`
	Resources       []Resource            `json:"resources"`
	Insights        Insights              `json:"insights"`
	Images          []string              `json:"images"`
	Ports           []int                 `json:"ports"`
	EnvVarKeys      []string              `json:"envVarKeys"`
	ConfigMapRefs   []string              `json:"configMapRefs"`
	SecretRefs      []string              `json:"secretRefs"`
	ServiceMappings []string              `json:"serviceMappings"`
	IngressRules    []string              `json:"ingressRules"`
	Snapshots       []ApplicationSnapshot `json:"snapshots"`
	Rollbacks       []RollbackEntry       `json:"rollbacks,omitempty"`
	Metrics         ApplicationMetrics    `json:"metrics"`
	CRStatus        string                `json:"crStatus,omitempty"`
	History         ApplicationHistory    `json:"history"`
}

type Health struct {
	Status        string  `json:"status"` // healthy | degraded | down | unknown
	Reason        *string `json:"reason"` // null when healthy, e.g. "2/3 replicas ready" when degraded
	ReadyReplicas int     `json:"readyReplicas"`
	TotalReplicas int     `json:"totalReplicas"`
}

type Namespaces struct {
	Total int              `json:"total"`
	Items []NamespaceEntry `json:"items"`
}

type NamespaceEntry struct {
	Name          string `json:"name"`
	ResourceCount int    `json:"resourceCount"`
}

type ResourceSummary struct {
	Deployment              int `json:"Deployment"`
	StatefulSet             int `json:"StatefulSet"`
	DaemonSet               int `json:"DaemonSet"`
	Job                     int `json:"Job"`
	CronJob                 int `json:"CronJob"`
	Service                 int `json:"Service"`
	NetworkPolicy           int `json:"NetworkPolicy"`
	Ingress                 int `json:"Ingress"`
	ServiceAccount          int `json:"ServiceAccount"`
	ConfigMap               int `json:"ConfigMap"`
	Secret                  int `json:"Secret"`
	PersistentVolumeClaim   int `json:"PersistentVolumeClaim"`
	HorizontalPodAutoscaler int `json:"HorizontalPodAutoscaler"`
	VerticalPodAutoscaler   int `json:"VerticalPodAutoscaler"`
}

type Resource struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

type Insights struct {
	Enriched      bool         `json:"enriched"`
	EnrichedAt    *string      `json:"enrichedAt"`
	Confidence    *string      `json:"confidence"`
	Summary       *string      `json:"summary"`
	TechStack     []string     `json:"techStack"`
	Role          *string      `json:"role"`
	Dependencies  []string     `json:"dependencies"`
	Category      *string      `json:"category"` // infrastructure | application | data | messaging | security
	Risks         []string     `json:"risks"`
	Suggestions   []string     `json:"suggestions"`
	RelatedApps   []RelatedApp `json:"relatedApps"`
	PromptVersion *string      `json:"promptVersion"`
}

type RelatedApp struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ApplicationHistory struct {
	Generation     int              `json:"generation"`
	HasDrift       bool             `json:"hasDrift"`
	LastModifiedBy string           `json:"lastModifiedBy,omitempty"`
	LastModifiedAt string           `json:"lastModifiedAt,omitempty"`
	ChangeLog      []ChangeLogEntry `json:"changeLog"`
}

type ChangeLogEntry struct {
	Generation  int                 `json:"generation"`
	DetectedAt  string              `json:"detectedAt"`
	ChangeClass string              `json:"changeClass"`
	Severity    string              `json:"severity"`
	ChangedBy   string              `json:"changedBy,omitempty"`
	Fingerprint string              `json:"fingerprint"`
	IsIncident  bool                `json:"isIncident"`
	IsRecovery  bool                `json:"isRecovery"`
	Changes     []ApplicationChange `json:"changes"`
	// API-computed convenience field; not part of the Application CRD.
	IsLastOne bool `json:"isLastOne,omitempty"`
}

type Managed struct {
	By      string  `json:"by"`
	Chart   *string `json:"chart"`
	Version *string `json:"version"`
}

type ApplicationChange struct {
	Field       string  `json:"field"`
	Description string  `json:"description"`
	ChangeType  string  `json:"changeType"` // "added" | "removed" | "updated"
	OldValue    *string `json:"oldValue"`
	NewValue    *string `json:"newValue"`
}

type ApplicationSnapshot struct {
	Generation  int    `json:"generation"`
	ChangeClass string `json:"changeClass"`
	Severity    string `json:"severity"`
	TakenAt     string `json:"takenAt"`
	ID          string `json:"id"`
	Namespace   string `json:"namespace"`
	Path        string `json:"path"`
}

type ResponseData struct {
	TotalResources    int           `json:"totalResources"`
	TotalApplications int           `json:"totalApplications"`
	Applications      []Application `json:"applications"`
}

const (
	// CRStatus is the status of the Application CR (created via NATS/notifier).
	CRStatusPending   = "Pending"
	CRStatusPublished = "Published"
	CRStatusCreated   = "Created"
	CRStatusFailed    = "Failed"

	// ChangeClass
	ChangeClassTopology   = "topology"
	ChangeClassDeployment = "deployment"
	ChangeClassScaling    = "scaling"
	ChangeClassConfig     = "config"
	ChangeClassResources  = "resources"
	ChangeClassDrift      = "drift"
	ChangeClassIncident   = "incident"
	ChangeClassRecovery   = "recovery"
	ChangeClassInitial    = "initial"

	// Severity
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
)
