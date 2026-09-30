package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/telark/telark/internal/data/plans"
	globalshared "github.com/telark/telark/internal/data/shared"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/applications"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
)

func RunKey(plan *plans.ProtectionPlan) (string, error) {
	if plan.StartedAt == nil {
		return constants.EmptyString, fmt.Errorf(string(ErrRunKeyMissingStart), plan.ID)
	}
	started, err := time.Parse(globalshared.DefaultTimeFormat, *plan.StartedAt)
	if err != nil {
		return constants.EmptyString, fmt.Errorf(string(ErrRunKeyInvalidStart), plan.ID, *plan.StartedAt, err)
	}
	return started.UTC().Format(RunKeyLayout), nil
}

func CapMessage(s string) string {
	if utf8.RuneCountInString(s) <= MaxMessageRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:MaxMessageRunes]) + MessageEllipsis
}

// RFC3339 UTC timestamps compare lexically.
func withinRun(ts string, plan *plans.ProtectionPlan) bool {
	if ts < *plan.StartedAt {
		return false
	}
	return plan.TerminatedAt == nil || ts <= *plan.TerminatedAt
}

func rowKey(v *planseps.ProtectionPlanViolation) string {
	if v.EventUID != constants.EmptyString {
		return v.EventUID
	}
	return strings.Join([]string{v.Timestamp, v.Policy, v.Rule, v.Namespace, v.Resource.Name}, keySeparator)
}

func Merge(
	existing, live []planseps.ProtectionPlanViolation,
	plan *plans.ProtectionPlan,
	limit int,
) ([]planseps.ProtectionPlanViolation, bool) {
	keyed := make(map[string]planseps.ProtectionPlanViolation, len(existing)+len(live))
	for i := range existing {
		keyed[rowKey(&existing[i])] = existing[i]
	}
	for i := range live {
		if !withinRun(live[i].Timestamp, plan) {
			continue
		}
		keyed[rowKey(&live[i])] = live[i]
	}
	rows := slices.Collect(maps.Values(keyed))
	for i := range rows {
		rows[i].Message = CapMessage(rows[i].Message)
	}
	total, page := violations.Page(rows, limit)
	return page, total > limit
}

// A ledger from a previous run starts over: that run is preserved in its own final report.
func (g *Generator) loadLedger(plan *plans.ProtectionPlan) (*PlanReportLedger, error) {
	run, err := RunKey(plan)
	if err != nil {
		return nil, err
	}
	empty := &PlanReportLedger{PlanID: plan.ID, Run: run}
	raw, err := g.store.GetLedger(plan.ID)
	if err != nil {
		return nil, err
	}
	if len(raw) == constants.DefaultInitValue {
		return empty, nil
	}
	var ledger PlanReportLedger
	if json.Unmarshal(raw, &ledger) != nil || ledger.Run != run {
		return empty, nil
	}
	return &ledger, nil
}

func policyNames(plan *plans.ProtectionPlan, ledger *PlanReportLedger) []string {
	if len(plan.RenderedPolicies) > constants.DefaultInitValue {
		return plan.RenderedPolicies
	}
	return ledger.RenderedPolicies
}

func (g *Generator) pastRetention(plan *plans.ProtectionPlan) bool {
	if plan.TerminatedAt == nil {
		return false
	}
	ended, err := time.Parse(globalshared.DefaultTimeFormat, *plan.TerminatedAt)
	return err != nil || g.clock().Sub(ended) >= violations.RetentionWindowDuration
}

// mergeLive folds what the cluster still holds into the ledger in memory; it never writes.
func (g *Generator) mergeLive(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	ledger *PlanReportLedger,
) (*PlanReportLedger, error) {
	names := policyNames(plan, ledger)
	if len(names) == constants.DefaultInitValue || g.pastRetention(plan) {
		return ledger, nil
	}
	namespaces, _, err := g.resolveLenient(ctx, plan)
	if err != nil {
		return nil, err
	}
	live, err := violations.Collect(ctx, g.dyn, namespaces, names, constants.EmptyString)
	if err != nil {
		return nil, err
	}
	now := g.clock().Format(globalshared.DefaultTimeFormat)
	ledger.Violations, ledger.Truncated = Merge(ledger.Violations, live, plan, g.maxViolations)
	ledger.RenderedPolicies = names
	ledger.Checkpoints = append(ledger.Checkpoints, LedgerCheckpoint{At: now, Health: plan.Health, ViolationsSeen: len(live)})
	ledger.UpdatedAt = now
	return ledger, nil
}

func (g *Generator) resolveLenient(
	ctx context.Context,
	plan *plans.ProtectionPlan,
) (namespaces, unresolved []string, err error) {
	if plan.Scope.Type == plans.ScopeTypeNamespaces {
		return plan.Scope.Namespaces, nil, nil
	}
	resolved, missing, err := g.resolveApps(ctx, plan.Scope.ApplicationRefs)
	if err != nil {
		return nil, nil, err
	}
	return applications.Namespaces(resolved, plan.Scope.ApplicationRefs), missing, nil
}

// One NX acquire, no wait: a busy lock skips only the write, never the merge.
func (g *Generator) persist(ctx context.Context, ledger *PlanReportLedger) {
	if g.rdb != nil {
		lock := xwareredis.NewLockClient(g.rdb)
		key := constants.KeyPrefixReportLedgerLock + ledger.PlanID
		value := uuid.NewString()
		acquired, err := lock.Acquire(ctx, key, value, constants.DefaultLockTTL)
		if err != nil || !acquired {
			g.logger.Info(fmt.Sprintf(LogLedgerLocked, ledger.PlanID))
			return
		}
		defer func() { _ = lock.Release(context.Background(), key, value) }()
	}
	raw, err := json.Marshal(ledger)
	if err == nil {
		err = g.store.PutLedger(ledger.PlanID, raw)
	}
	if err != nil {
		g.logger.Error(fmt.Sprintf(LogLedgerWriteFailed, ledger.PlanID, err))
	}
}

// Checkpoint is the leader loop's call: merge what the cluster still holds and persist it.
func (g *Generator) Checkpoint(ctx context.Context, plan *plans.ProtectionPlan) (ledger *PlanReportLedger, err error) {
	err = g.guarded(func() error {
		loaded, loadErr := g.loadLedger(plan)
		if loadErr != nil {
			return loadErr
		}
		merged, mergeErr := g.mergeLive(ctx, plan, loaded)
		if mergeErr != nil {
			return mergeErr
		}
		g.persist(ctx, merged)
		ledger = merged
		return nil
	})
	return ledger, err
}
