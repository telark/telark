package application

import "time"

type Resource struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
}

type NamespaceEntry struct {
	Name          string `json:"name"`
	ResourceCount int    `json:"resourceCount"`
}

type Namespaces struct {
	Total int              `json:"total"`
	Items []NamespaceEntry `json:"items"`
}

type Managed struct {
	By      string  `json:"by"`
	Chart   *string `json:"chart"`
	Version *string `json:"version"`
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

// RelatedApp describes another application this app depends on (from LLM).
type RelatedApp struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type Health struct {
	Status        string  `json:"status"` // healthy | degraded | down | unknown
	Reason        *string `json:"reason"` // null when healthy, e.g. "2/3 replicas ready" when degraded
	ReadyReplicas int     `json:"readyReplicas"`
	TotalReplicas int     `json:"totalReplicas"`
}

type Insights struct {
	Enriched      bool         `json:"enriched"`
	EnrichedAt    *time.Time   `json:"enrichedAt"`
	Confidence    *string      `json:"confidence"`
	Summary       *string      `json:"summary"`
	TechStack     []string     `json:"techStack"`
	Role          *string      `json:"role"`
	Dependencies  []string     `json:"dependencies"`
	Category      *string      `json:"category"`    // infrastructure | application | data | messaging | security
	Risks         []string     `json:"risks"`       
	Suggestions   []string     `json:"suggestions"` 
	RelatedApps   []RelatedApp `json:"relatedApps"`
	PromptVersion *string      `json:"promptVersion"`
}

// CRStatus is the status of the Application CR (created via NATS/notifier).
const (
	CRStatusPending   = "Pending"
	CRStatusPublished = "Published"
	CRStatusCreated   = "Created"
	CRStatusFailed    = "Failed"
)

type Application struct {
	Name            string          `json:"name"`
	DisplayName     string          `json:"displayName"`
	Health          Health          `json:"health"`
	ResourceCount   int             `json:"resourceCount"`
	Namespaces      Namespaces      `json:"namespaces"`
	Managed         Managed         `json:"managed"`
	CreatedAt       time.Time       `json:"createdAt"`
	LastUpdated     time.Time       `json:"lastUpdated"`
	ResourceSummary ResourceSummary `json:"resourceSummary"`
	Resources       []Resource      `json:"resources"`
	Insights        Insights        `json:"insights"`
	Images          []string        `json:"images"`
	Ports           []int           `json:"ports"`
	EnvVarKeys      []string        `json:"envVarKeys"`
	// CRStatus is set after publishing to NATS: Published (ack received), Failed (publish error), or Created (when notifier confirms).
	CRStatus string `json:"crStatus,omitempty"`
}

type ResponseData struct {
	TotalResources    int           `json:"totalResources"`
	TotalApplications int           `json:"totalApplications"`
	Applications      []Application `json:"applications"`
}

type APIResponse struct {
	Status    int          `json:"status"`
	Operation string       `json:"operation"`
	Message   string       `json:"message"`
	Data      ResponseData `json:"data"`
}
