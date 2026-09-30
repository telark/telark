package insightsindex

import (
	"sync"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/resources/application"
)

// One insight card, flattened for the cluster-wide list; the full card stays in the app's document.
type Row struct {
	ID string `json:"id"`
	// The document's namespace, the app address for triage, Analyze and doc reads; filters match WorkloadNamespace.
	Namespace         string                     `json:"namespace"`
	WorkloadNamespace string                     `json:"workloadNamespace"`
	App               string                     `json:"app"`
	Category          string                     `json:"category"`
	Kind              string                     `json:"kind"`
	Reason            string                     `json:"reason,omitempty"`
	Subject           string                     `json:"subject"`
	Title             string                     `json:"title"`
	Severity          string                     `json:"severity"`
	Confidence        string                     `json:"confidence"`
	Status            string                     `json:"status"`
	Stale             bool                       `json:"stale"`
	Triage            *application.InsightTriage `json:"triage,omitempty"`
	FirstSeenAt       string                     `json:"firstSeenAt"`
	LastSeenAt        string                     `json:"lastSeenAt"`
	ResolvedAt        string                     `json:"resolvedAt,omitempty"`
	Environments      []string                   `json:"environments"`

	lastSeenMs int64
	// Unix ms after which an active row is stale; zero for resolved or undated rows.
	staleAtMs int64
	haystack  string
}

type Counts struct {
	BySeverity map[string]int `json:"bySeverity"`
	ByCategory map[string]int `json:"byCategory"`
	ByState    map[string]int `json:"byState"`
	// BySeverity with the severity filter left out, so quick-filter pills keep every count.
	SeverityFacet map[string]int `json:"severityFacet"`
}

type Page struct {
	Items     []Row  `json:"items"`
	Total     int    `json:"total"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	Counts    Counts `json:"counts"`
	IndexedAt string `json:"indexedAt"`
}

// Normalized: lists are sorted and deduplicated, so equal filters hash to one ETag.
type Query struct {
	Category    string
	Kinds       []string
	Severities  []string
	States      []string
	Triage      string
	Namespaces  []string
	Environment string
	Search      string
	IDs         []string
	// "<namespace>/<name>" members.
	Apps     []string
	Page     int
	PageSize int
	// Read-your-writes: catch up with Redis before answering. Not a filter, so not in the ETag.
	Fresh bool
}

type Settings struct {
	Refresh    time.Duration
	Resync     time.Duration
	StaleAfter time.Duration
}

type PlanLister func() ([]plans.ProtectionPlan, error)

type Index struct {
	mu sync.RWMutex
	// Serializes Redis reads so an older read never overwrites a newer one's rows.
	syncMu   sync.Mutex
	settings Settings
	// Keyed by the index member "<namespace>/<name>"; a slice is replaced, never edited in place.
	rows   map[string][]Row
	scores map[string]float64
	// Hashed into every ETag so two replicas never hand out one tag for different content.
	epoch     int64
	version   uint64
	lastScore float64
	loaded    bool
	indexedAt time.Time
	// Sorted staleAtMs of every active row: how many lie behind now changes exactly when a row goes stale.
	staleAtMs []int64
	envByApp  map[string][]string
	envByNS   map[string][]string
}
