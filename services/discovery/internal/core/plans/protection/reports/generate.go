package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	reportseps "github.com/telark/rest/endpoints/reports"
	"k8s.io/client-go/dynamic"
)

func NewGenerator(
	store ReportStore,
	dyn dynamic.Interface,
	resolveApps applications.Resolver,
	rdb *redis.Client,
	logger Logger,
	maxViolations int,
) *Generator {
	return &Generator{
		store:         store,
		dyn:           dyn,
		resolveApps:   resolveApps,
		rdb:           rdb,
		clock:         func() time.Time { return time.Now().UTC() },
		logger:        logger,
		maxViolations: maxViolations,
		sem:           make(chan struct{}, MaxConcurrentRenders),
		captureSlot:   make(chan struct{}, CaptureSlots),
	}
}

// A panic in gather/render becomes an error so the sequential controller never crash-loops.
func (g *Generator) guarded(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			g.logger.Error(fmt.Sprintf(LogReportPanic, r))
			err = fmt.Errorf(string(constants.ErrReportPanic), r)
		}
	}()
	return fn()
}

func (g *Generator) Generate(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	actor, trigger string,
) (meta *reportseps.ReportMeta, err error) {
	err = g.guarded(func() error {
		if trigger == reportseps.TriggerManual {
			select {
			case g.sem <- struct{}{}:
			default:
				return ErrRenderBusy
			}
			defer func() { <-g.sem }()
		}
		generated, genErr := g.build(ctx, plan, actor, trigger)
		meta = generated
		return genErr
	})
	return meta, err
}

// Manual renders are read-only; only boundary captures (and the leader loop) persist the ledger.
func (g *Generator) build(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	actor, trigger string,
) (*reportseps.ReportMeta, error) {
	ledger, err := g.loadLedger(plan)
	if err != nil {
		return nil, err
	}
	if ledger, err = g.mergeLive(ctx, plan, ledger); err != nil {
		return nil, err
	}
	if trigger != reportseps.TriggerManual {
		g.persist(ctx, ledger)
	}
	doc, err := g.Gather(ctx, plan, ledger, actor, trigger, g.clock())
	if err != nil {
		return nil, err
	}
	files, err := RenderWithinBudget(doc, constants.ReportRequestBudgetBytes)
	if err != nil {
		return nil, err
	}
	meta, err := g.store.Create(reportseps.CreatePlanReportRequest{Meta: metaOf(doc), Files: files})
	if err != nil {
		return nil, err
	}
	if meta.GeneratedAt != doc.Cover.GeneratedAt {
		g.logger.Info(fmt.Sprintf(LogReportReplayed, plan.ID, meta.ID))
	}
	return meta, nil
}

// CaptureAsync renders the boundary report from an in-memory copy of the plan stamped with the
// terminal fields, after the terminal patch, without ever blocking the caller.
func (g *Generator) CaptureAsync(plan *plans.ProtectionPlan, endedAt, actor, reason, trigger string) <-chan struct{} {
	done := make(chan struct{})
	if plan.Phase != plans.PhaseActive || plan.StartedAt == nil {
		close(done)
		return done
	}
	p := *plan
	p.TerminatedAt = &endedAt
	p.TerminatedBy = &actor
	p.Reason = &reason
	switch trigger {
	case reportseps.TriggerEnd:
		p.Phase = plans.PhaseTerminated
	case reportseps.TriggerCancel:
		p.Phase = plans.PhaseCanceled
	default:
	}
	go func() {
		defer close(done)
		g.captureSlot <- struct{}{}
		defer func() { <-g.captureSlot }()
		cctx, cancel := context.WithTimeout(context.Background(), constants.DefaultReportCaptureTimeout)
		defer cancel()
		meta, err := g.Generate(cctx, &p, actor, trigger)
		if err != nil {
			g.logger.Error(fmt.Sprintf(LogReportCaptureFailed, p.ID, trigger, err))
			return
		}
		g.logger.Info(fmt.Sprintf(LogReportCaptured, p.ID, meta.ID))
	}()
	return done
}

func ManualAllowed(plan *plans.ProtectionPlan) error {
	if plan.Phase == plans.PhaseDraft || plan.Phase == plans.PhaseScheduled {
		return validation.Invalidf(string(ErrReportInvalidPhase), plan.Phase)
	}
	if plan.StartedAt == nil {
		return validation.Invalidf(string(ErrReportNotStarted), plan.ID)
	}
	return nil
}
