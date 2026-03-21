package application

import "time"

type Application struct {
	Name            string          `json:"name"`
	DisplayName     string          `json:"displayName"`
	Health          Health          `json:"health"`
	ResourceCount   int             `json:"resourceCount"`
	Namespaces      Namespaces      `json:"namespaces"`
	Managed         Managed         `json:"managed"`
	CreatedAt       string          `json:"createdAt"`
	LastUpdated     string          `json:"lastUpdated"`
	ResourceSummary ResourceSummary `json:"resourceSummary"`
	Resources       []Resource      `json:"resources"`
	Insights        Insights        `json:"insights"`
	Images          []string        `json:"images"`
	Ports           []int           `json:"ports"`
	EnvVarKeys      []string        `json:"envVarKeys"`
	// CRStatus is set after publishing to NATS: Published (ack received), Failed (publish error), or Created (when notifier confirms).
	CRStatus string `json:"crStatus,omitempty"`
	// Snapshot references — top level, not inside history.
	// Ordered by generation ascending.
	// Max SNAPSHOTS_MAX_VERSIONS entries (retention applied).
	// Never nil — initialize as []ApplicationSnapshot{}.
	Snapshots []ApplicationSnapshot `json:"snapshots"`
	History   ApplicationHistory    `json:"history"`
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
	EnrichedAt    *time.Time   `json:"enrichedAt"`
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
	// Monotonically incrementing — never resets.
	Generation int `json:"generation"`
	// Whether current state differs from last stored.
	HasDrift bool `json:"hasDrift"`
	// Classification of the LATEST change set only; empty when hasDrift is false.
	ChangeClass string `json:"changeClass"`
	// Severity of the LATEST change set only.
	Severity string `json:"severity"`
	// Source of the LATEST change.
	Source string `json:"source"`
	// True if latest change is an incident.
	IsIncident bool `json:"isIncident"`
	// True if latest change is a recovery.
	IsRecovery bool `json:"isRecovery"`
	// Fingerprint of the LATEST change set.
	Fingerprint string `json:"fingerprint"`
	// Number of field changes in LATEST diff.
	ChangeCount int `json:"changeCount"`
	// When the LATEST change was detected; RFC3339 when set, null for new app / no change.
	DetectedAt *string `json:"detectedAt"`
	// Individual field changes from the LATEST diff only; empty when hasDrift is false.
	// Never nil — initialize as []ApplicationChange{}.
	Changes []ApplicationChange `json:"changes"`
	// Full audit trail — one entry per generation that had a change, ordered by generation ascending.
	// Never trimmed — grows indefinitely. Empty for new applications.
	// Never nil — initialize as []ChangeLogEntry{}.
	ChangeLog []ChangeLogEntry `json:"changeLog"`
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

// ChangeLogEntry captures one generation's worth of changes as a historical record.
type ChangeLogEntry struct {
	// The generation this entry belongs to.
	Generation int `json:"generation"`
	// When this change was detected.
	DetectedAt string `json:"detectedAt"`
	// Classification of this change set.
	ChangeClass string `json:"changeClass"`
	// Severity of this change set.
	Severity string `json:"severity"`
	// Source of this change.
	Source string `json:"source"`
	// SHA fingerprint of this change set.
	Fingerprint string `json:"fingerprint"`
	// Whether this was an incident.
	IsIncident bool `json:"isIncident"`
	// Whether this was a recovery.
	IsRecovery bool `json:"isRecovery"`
	// The individual field changes in this generation.
	// Never nil — initialize as []ApplicationChange{}.
	Changes []ApplicationChange `json:"changes"`
}

type ApplicationSnapshot struct {
	Generation  int       `json:"generation"`
	ChangeClass string    `json:"changeClass"`
	Severity    string    `json:"severity"`
	TakenAt     time.Time `json:"takenAt"`
	ID          string    `json:"id"`
	Namespace   string    `json:"namespace"`
	Path        string    `json:"path"`
}

type ResponseData struct {
	TotalResources    int           `json:"totalResources"`
	TotalApplications int           `json:"totalApplications"`
	Applications      []Application `json:"applications"`
}

// CRStatus is the status of the Application CR (created via NATS/notifier).
const (
	CRStatusPending   = "Pending"
	CRStatusPublished = "Published"
	CRStatusCreated   = "Created"
	CRStatusFailed    = "Failed"
)
