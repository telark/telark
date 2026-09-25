package reports

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/telark/data/plans"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/violations"
	planseps "github.com/telark/rest/endpoints/plans"
	reportseps "github.com/telark/rest/endpoints/reports"
)

var enginePattern = regexp.MustCompile("(?i)" + regexp.QuoteMeta(engineName))

// Scrub replaces the engine's name wherever cluster-controlled free text could carry it.
func Scrub(s string) string {
	return enginePattern.ReplaceAllLiteralString(s, EngineReplacement)
}

func ReportID(plan *plans.ProtectionPlan, trigger string, generatedAt time.Time) (string, error) {
	run, err := RunKey(plan)
	if err != nil {
		return constants.EmptyString, err
	}
	id := run + idSeparator + trigger
	if trigger == reportseps.TriggerManual {
		id += idSeparator + generatedAt.UTC().Format(RunKeyLayout)
	}
	return id, nil
}

func (g *Generator) Gather(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	ledger *PlanReportLedger,
	actor, trigger string,
	generatedAt time.Time,
) (*ReportDocument, error) {
	reportID, err := ReportID(plan, trigger, generatedAt)
	if err != nil {
		return nil, err
	}
	namespaces, unresolved, err := g.resolveLenient(ctx, plan)
	if err != nil {
		return nil, err
	}
	at := generatedAt.UTC().Format(globalshared.DefaultTimeFormat)
	end := at
	if plan.TerminatedAt != nil {
		end = *plan.TerminatedAt
	}
	names := policyNames(plan, ledger)
	rows := scrubRows(ledger.Violations)
	doc := &ReportDocument{
		Cover:     cover(plan, reportID, actor, trigger, at, end),
		Coverage:  g.coverage(plan, ledger, end, unresolved),
		Timeline:  timeline(plan),
		Scope:     ScopeSection{Type: plan.Scope.Type, Namespaces: namespaces, ApplicationIDs: plan.Scope.ApplicationIDs},
		Policies:  policyBlocks(plan, names),
		Health:    HealthSection{Value: plan.Health, CheckedAt: deref(plan.HealthCheckedAt), Timeline: ledger.Checkpoints},
		Decisions: DecisionsSection{Aggregates: aggregate(rows), Rows: rows},
		Appendix: AppendixSection{
			Glossary:         []string{GlossaryAudit, GlossaryEnforce, GlossaryDecision},
			RenderedPolicies: names,
			PlanID:           plan.ID,
			ReportID:         reportID,
			SchemaVersion:    SchemaVersion,
			Generator:        GeneratorName,
			Notes:            []string{SentenceAppActivityExcluded},
		},
	}
	return doc, nil
}

func cover(plan *plans.ProtectionPlan, reportID, actor, trigger, at, end string) CoverSection {
	mode := ModeLabelAudit
	if plan.Mode == plans.ModeEnforce {
		mode = ModeLabelEnforce
	}
	section := CoverSection{
		Name:        plan.Name,
		ID:          plan.ID,
		Description: Scrub(deref(plan.Description)),
		Severity:    plan.Severity,
		Priority:    plan.Priority,
		Mode:        mode,
		TimeMode:    plan.TimeMode,
		Phase:       plan.Phase,
		Reason:      Scrub(deref(plan.Reason)),
		ReportID:    reportID,
		Trigger:     trigger,
		GeneratedAt: at,
		GeneratedBy: actor,
		RunStart:    deref(plan.StartedAt),
		RunEnd:      end,
	}
	if plan.TimeRange != nil {
		section.StartAt, section.EndAt = plan.TimeRange.StartAt, plan.TimeRange.EndAt
	}
	return section
}

func (g *Generator) coverage(plan *plans.ProtectionPlan, ledger *PlanReportLedger, end string, unresolved []string) CoverageSection {
	live := len(plan.RenderedPolicies) > constants.DefaultInitValue
	section := CoverageSection{
		RetentionWindow:        violations.RetentionWindow,
		CheckpointInterval:     g.checkpointEvery.String(),
		Checkpoints:            len(ledger.Checkpoints),
		Gaps:                   gaps(plan, ledger, end),
		Truncated:              ledger.Truncated,
		RowCap:                 g.maxViolations,
		PoliciesLiveAtCapture:  live,
		UnresolvedApplications: unresolved,
		Provenance:             provenance(live, ledger),
		Notes: []string{
			fmt.Sprintf(SentenceRetention, violations.RetentionWindow),
			SentenceCountsAreFloors,
			fmt.Sprintf(SentenceMessagesCapped, MaxMessageRunes),
			SentenceNoRequester,
		},
	}
	if n := len(ledger.Checkpoints); n > constants.DefaultInitValue {
		section.FirstCheckpoint = ledger.Checkpoints[constants.DefaultInitValue].At
		section.LastCheckpoint = ledger.Checkpoints[n-constants.DefaultAddValue].At
	}
	if ledger.Truncated {
		section.Notes = append(section.Notes, fmt.Sprintf(SentenceRowCap, g.maxViolations))
	}
	if !live && plan.TerminatedAt != nil {
		section.Notes = append(section.Notes, SentencePostEnd)
	}
	return section
}

func provenance(live bool, ledger *PlanReportLedger) []ProvenanceRow {
	policySource := ProvenanceLive
	switch {
	case live:
	case len(ledger.RenderedPolicies) > constants.DefaultInitValue:
		policySource = ProvenanceFromLedger
	default:
		policySource = ProvenanceNotAvailable
	}
	decisions := ProvenanceFromLedger
	if len(ledger.Violations) == constants.DefaultInitValue {
		decisions = ProvenanceNotAvailable
	}
	return []ProvenanceRow{
		{Section: TitleCover, Source: ProvenanceStoredOnPlan},
		{Section: TitleTimeline, Source: ProvenanceStoredOnPlan},
		{Section: TitleScope, Source: ProvenanceStoredOnPlan},
		{Section: TitlePolicies, Source: policySource},
		{Section: TitleHealth, Source: policySource},
		{Section: TitleDecisions, Source: decisions},
	}
}

func between(from, to string) (time.Duration, bool) {
	start, err := time.Parse(globalshared.DefaultTimeFormat, from)
	if err != nil {
		return constants.DefaultInitValue, false
	}
	stop, err := time.Parse(globalshared.DefaultTimeFormat, to)
	if err != nil {
		return constants.DefaultInitValue, false
	}
	return stop.Sub(start), true
}

// Every interval longer than the retention window is a stretch the ledger could not have covered.
func gaps(plan *plans.ProtectionPlan, ledger *PlanReportLedger, end string) []Gap {
	points := make([]string, constants.DefaultInitValue, len(ledger.Checkpoints)+gapEndpoints)
	points = append(points, deref(plan.StartedAt))
	for _, cp := range ledger.Checkpoints {
		points = append(points, min(cp.At, end))
	}
	points = append(points, end)
	var out []Gap
	for i := constants.DefaultAddValue; i < len(points); i++ {
		gap, ok := between(points[i-constants.DefaultAddValue], points[i])
		if ok && gap > violations.RetentionWindowDuration {
			out = append(out, Gap{From: points[i-constants.DefaultAddValue], To: points[i]})
		}
	}
	return out
}

func timeline(plan *plans.ProtectionPlan) TimelineSection {
	return TimelineSection{
		CreatedAt:       plan.CreatedAt,
		CreatedBy:       plan.CreatedBy,
		StartedAt:       deref(plan.StartedAt),
		StartedBy:       deref(plan.StartedBy),
		LastUpdatedAt:   plan.LastUpdatedAt,
		LastUpdatedBy:   plan.LastUpdatedBy,
		TerminatedAt:    deref(plan.TerminatedAt),
		TerminatedBy:    deref(plan.TerminatedBy),
		ParticipantsIDs: plan.ParticipantsIDs,
	}
}

// Specs and rendered names are joined by position; a rendered name past the spec list still gets a block.
func policyBlocks(plan *plans.ProtectionPlan, names []string) []PolicyBlock {
	blocks := make([]PolicyBlock, max(len(plan.Policies), len(names)))
	for i, spec := range plan.Policies {
		blocks[i].TemplateID = spec.TemplateID
		if params, err := json.Marshal(spec.Params); err == nil && len(spec.Params) > constants.DefaultInitValue {
			blocks[i].Params = string(params)
		}
	}
	for i, name := range names {
		blocks[i].RenderedName = name
		for _, detail := range plan.HealthDetail {
			if detail.PolicyName == name {
				blocks[i].Present, blocks[i].Ready, blocks[i].FailureAction = detail.Present, detail.Ready, detail.FailureAction
			}
		}
	}
	return blocks
}

func scrubRows(rows []planseps.ProtectionPlanViolation) []planseps.ProtectionPlanViolation {
	out := slices.Clone(rows)
	for i := range out {
		out[i].Message = Scrub(out[i].Message)
		out[i].Rule = Scrub(out[i].Rule)
	}
	return out
}

func aggregate(rows []planseps.ProtectionPlanViolation) Aggregates {
	byResult, byPolicyRule, byNamespace, byKind := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	resources := map[string]struct{}{}
	agg := Aggregates{Total: len(rows)}
	for i := range rows {
		r := &rows[i]
		byResult[r.Result]++
		byPolicyRule[r.Policy+idSeparator+r.Rule]++
		byNamespace[r.Namespace]++
		byKind[r.Resource.Kind]++
		resources[strings.Join([]string{r.Resource.Kind, r.Resource.Namespace, r.Resource.Name}, keySeparator)] = struct{}{}
		if agg.First == constants.EmptyString || r.Timestamp < agg.First {
			agg.First = r.Timestamp
		}
		agg.Last = max(agg.Last, r.Timestamp)
	}
	agg.ByResult, agg.ByPolicyRule = countRows(byResult), countRows(byPolicyRule)
	agg.ByNamespace, agg.ByKind = countRows(byNamespace), countRows(byKind)
	agg.DistinctResources = len(resources)
	return agg
}

func countRows(counts map[string]int) []CountRow {
	out := make([]CountRow, constants.DefaultInitValue, len(counts))
	for key, count := range counts {
		out = append(out, CountRow{Key: key, Count: count})
	}
	slices.SortFunc(out, func(a, b CountRow) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Key, b.Key))
	})
	return out
}

func deref(s *string) string {
	if s == nil {
		return constants.EmptyString
	}
	return *s
}
