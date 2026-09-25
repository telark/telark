package reports

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	planseps "github.com/telark/rest/endpoints/plans"
	reportseps "github.com/telark/rest/endpoints/reports"
	"k8s.io/client-go/dynamic"
)

var ErrRenderBusy = errors.New(renderBusyMessage)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type ReportStore interface {
	Create(req reportseps.CreatePlanReportRequest) (*reportseps.ReportMeta, error)
	PutLedger(planID string, ledger json.RawMessage) error
	GetLedger(planID string) (json.RawMessage, error)
}

type PlanReportLedger struct {
	PlanID           string                             `json:"planId"`
	Run              string                             `json:"run"`
	UpdatedAt        string                             `json:"updatedAt"`
	RenderedPolicies []string                           `json:"renderedPolicies"`
	Checkpoints      []LedgerCheckpoint                 `json:"checkpoints"`
	Violations       []planseps.ProtectionPlanViolation `json:"violations"`
	Truncated        bool                               `json:"truncated"`
}

type LedgerCheckpoint struct {
	At             string `json:"at"`
	Health         string `json:"health"`
	ViolationsSeen int    `json:"violationsSeen"`
}

type Generator struct {
	store         ReportStore
	dyn           dynamic.Interface
	resolveApps   applications.Resolver
	rdb           *redis.Client
	clock         func() time.Time
	logger        Logger
	maxViolations int
	// The configured cadence, not the ledger's observed spacing: a report adds a checkpoint of its own.
	checkpointEvery time.Duration
	sem             chan struct{}
	captureSlot     chan struct{}
}

type ReportDocument struct {
	Cover     CoverSection     `json:"cover"`
	Coverage  CoverageSection  `json:"coverage"`
	Timeline  TimelineSection  `json:"timeline"`
	Scope     ScopeSection     `json:"scope"`
	Policies  []PolicyBlock    `json:"policies"`
	Health    HealthSection    `json:"health"`
	Decisions DecisionsSection `json:"decisions"`
	Appendix  AppendixSection  `json:"appendix"`
}

type CoverSection struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Priority    int    `json:"priority"`
	Mode        string `json:"mode"`
	TimeMode    string `json:"timeMode"`
	StartAt     string `json:"startAt"`
	EndAt       string `json:"endAt"`
	Phase       string `json:"phase"`
	Reason      string `json:"reason"`
	ReportID    string `json:"reportId"`
	Trigger     string `json:"trigger"`
	GeneratedAt string `json:"generatedAt"`
	GeneratedBy string `json:"generatedBy"`
	RunStart    string `json:"runStart"`
	RunEnd      string `json:"runEnd"`
}

type CoverageSection struct {
	RetentionWindow        string          `json:"retentionWindow"`
	CheckpointInterval     string          `json:"checkpointInterval"`
	Checkpoints            int             `json:"checkpoints"`
	FirstCheckpoint        string          `json:"firstCheckpoint"`
	LastCheckpoint         string          `json:"lastCheckpoint"`
	Gaps                   []Gap           `json:"gaps"`
	Truncated              bool            `json:"truncated"`
	RowsOmitted            int             `json:"rowsOmitted"`
	RowCap                 int             `json:"rowCap"`
	PoliciesLiveAtCapture  bool            `json:"policiesLiveAtCapture"`
	UnresolvedApplications []string        `json:"unresolvedApplications"`
	Provenance             []ProvenanceRow `json:"provenance"`
	Notes                  []string        `json:"notes"`
}

type Gap struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ProvenanceRow struct {
	Section string `json:"section"`
	Source  string `json:"source"`
}

type TimelineSection struct {
	CreatedAt       string   `json:"createdAt"`
	CreatedBy       string   `json:"createdBy"`
	StartedAt       string   `json:"startedAt"`
	StartedBy       string   `json:"startedBy"`
	LastUpdatedAt   string   `json:"lastUpdatedAt"`
	LastUpdatedBy   string   `json:"lastUpdatedBy"`
	TerminatedAt    string   `json:"terminatedAt"`
	TerminatedBy    string   `json:"terminatedBy"`
	ParticipantsIDs []string `json:"participantsIDs"`
}

type ScopeSection struct {
	Type           string   `json:"type"`
	Namespaces     []string `json:"namespaces"`
	ApplicationIDs []string `json:"applicationIds"`
}

type PolicyBlock struct {
	TemplateID    string `json:"templateID"`
	Params        string `json:"params"`
	RenderedName  string `json:"renderedName"`
	Present       bool   `json:"present"`
	Ready         bool   `json:"ready"`
	FailureAction string `json:"failureAction"`
}

type HealthSection struct {
	Value     string             `json:"value"`
	CheckedAt string             `json:"checkedAt"`
	Timeline  []LedgerCheckpoint `json:"timeline"`
}

type DecisionsSection struct {
	Aggregates Aggregates                         `json:"aggregates"`
	Rows       []planseps.ProtectionPlanViolation `json:"rows"`
}

type Aggregates struct {
	Total             int        `json:"total"`
	ByResult          []CountRow `json:"byResult"`
	ByPolicyRule      []CountRow `json:"byPolicyRule"`
	ByNamespace       []CountRow `json:"byNamespace"`
	ByKind            []CountRow `json:"byKind"`
	First             string     `json:"first"`
	Last              string     `json:"last"`
	DistinctResources int        `json:"distinctResources"`
}

type CountRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type AppendixSection struct {
	Glossary         []string `json:"glossary"`
	RenderedPolicies []string `json:"renderedPolicies"`
	PlanID           string   `json:"planId"`
	ReportID         string   `json:"reportId"`
	SchemaVersion    string   `json:"schemaVersion"`
	Generator        string   `json:"generator"`
	Notes            []string `json:"notes"`
}

// Labels carries every fixed title and sentence into the templates as data.
type Labels struct {
	TitleCover, TitleCoverage, TitleTimeline, TitleScope string
	TitlePolicies, TitleHealth, TitleDecisions           string
	TitleAppendix, NotAvailable                          string
}

type templateData struct {
	Doc *ReportDocument
	L   Labels
}
